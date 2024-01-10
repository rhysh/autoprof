package autoprof

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/url"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"sync"
	"time"
)

// NewZipCollector returns a Collector which will write out a profile bundle
// formatted as a zip archive to the provided io.Writer.
func NewZipCollector(w io.Writer, meta *ArchiveMeta, opt *ArchiveOptions) *Collector {
	zw := zip.NewWriter(w)
	return &Collector{
		meta: meta,
		opt:  opt,
		writeFileHeader: func(name string) (io.Writer, error) {
			return zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		},
		finish: zw.Close,
	}
}

// ArchiveOptions holds user preferences and instructions for the collection
// of a profile bundle.
type ArchiveOptions struct {
	// CPUProfileDuration is the requested duration of the CPU profile. Leave
	// at 0 to disable CPU profiling.
	CPUProfileDuration time.Duration
	// CPUProfileByteTarget is an optional soft limit on the size of the CPU
	// profile. The collector will stop the profile as soon as possible after
	// it reaches the size target. When unset, there is no limit.
	CPUProfileByteTarget int64

	// ExecutionTraceDuration is the requested duration of the execution
	// trace. Leave at 0 to disable execution tracing.
	ExecutionTraceDuration time.Duration
	// ExecutionTraceByteTarget is an optional soft limit on the size of the
	// execution trace output. The collector will stop the execution trace as
	// soon as possible after it reaches the size target. When unset, there is
	// no limit.
	ExecutionTraceByteTarget int64

	// CustomDataSources holds user-specified additional data sources. When
	// generating a zip-archived profile bundle, data from these sources will
	// be included in the "custom/" directory. The map key names will be URI
	// path-escaped and used to name the files within that directory.
	CustomDataSources map[string]*DataSource
}

// A DataSource can generate data to be included in a profile bundle.
type DataSource struct {
	WriteTo func(ctx context.Context, w io.Writer) error
}

// A Collector assembles and writes out a profile bundle. It cannot be reused.
type Collector struct {
	meta *ArchiveMeta
	opt  *ArchiveOptions
	// writeFileHeader prepares the profile bundle to receive data for a
	// record with the provided name.
	writeFileHeader func(name string) (io.Writer, error)
	// finish completes the profile bundle, indicating that no more data will
	// be written.
	finish func() error
	// addErr holds onto any error encountered while calling the add method
	// for delayed processing.
	addErr error
}

// add stores the data from source into the profile bundle, using the provided
// name. It tracks any errors that occur when adding data into the bundle, and
// returns early if any previous call encountered an error.
func (c *Collector) add(ctx context.Context, name string, source *DataSource) {
	if source == nil || source.WriteTo == nil {
		return
	}
	if c.addErr != nil {
		return
	}
	var w io.Writer
	w, c.addErr = c.writeFileHeader(name)
	if c.addErr != nil {
		return
	}
	c.addErr = source.WriteTo(ctx, w)
}

// Run collects the specified profile bundle.
func (c *Collector) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.add(ctx, "meta", metaSource(c.meta))
	c.add(ctx, "expvar", expvarSource())

	// write heap profile first, so it's in a consistent position
	c.add(ctx, "pprof/heap", pprofSource(pprof.Lookup("heap")))

	for _, profile := range pprof.Profiles() {
		if name := profile.Name(); name != "heap" {
			c.add(ctx, "pprof/"+url.PathEscape(name), pprofSource(profile))
		}
	}

	custom := make([]string, 0, len(c.opt.CustomDataSources))
	for name := range c.opt.CustomDataSources {
		custom = append(custom, name)
	}
	sort.Strings(custom)
	for _, name := range custom {
		c.add(ctx, "custom/"+url.PathEscape(name), c.opt.CustomDataSources[name])
	}

	if c.addErr != nil {
		return c.addErr
	}

	if c.opt.CPUProfileDuration > 0 {
		err := c.addCPUProfile(ctx, "pprof/profile")
		if err != nil {
			return err
		}
	}

	if c.opt.ExecutionTraceDuration > 0 {
		err := c.addExecutionTrace(ctx, "pprof/trace", "pprof/profile-during-trace")
		if err != nil {
			return err
		}
	}

	return c.finish()
}

func (c *Collector) addCPUProfile(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, c.opt.CPUProfileDuration)
	defer cancel()
	return c.addTimeBasedProfile(ctx, name, c.opt.CPUProfileByteTarget, pprof.StartCPUProfile, pprof.StopCPUProfile)
}

func (c *Collector) addExecutionTrace(ctx context.Context, name, profileName string) error {
	ctx, cancel := context.WithTimeout(ctx, c.opt.ExecutionTraceDuration)
	defer cancel()

	start := trace.Start
	stop := trace.Stop

	var cpuProfile *bytes.Buffer

	if c.opt.CPUProfileDuration > 0 {
		// CPU profiles are enabled for this bundle. Run a CPU profile that
		// wholly encompasses the execution trace, to make CPU samples appear in
		// the execution trace (new in Go 1.19).

		start = func(w io.Writer) error {
			// The CPU profile starts before the execution trace. The
			// runtime/pprof package aggregates the CPU profile in memory and
			// only writes out data during the StopCPUProfile call, so there's
			// no use in streaming the data to enforce a soft limit on its size.
			var buf bytes.Buffer
			if err := pprof.StartCPUProfile(&buf); err == nil {
				cpuProfile = &buf
			}

			return trace.Start(w)
		}
		stop = func() {
			trace.Stop()

			// The CPU profile stops after the execution trace. The execution
			// trace still has an open io.Writer to the zip archive; we'll need
			// to wait for that to finish before adding a new file.
			if cpuProfile != nil {
				pprof.StopCPUProfile()
			}
		}
	}

	traceErr := c.addTimeBasedProfile(ctx, name, c.opt.ExecutionTraceByteTarget, start, stop)

	profileErr := func() error {
		if cpuProfile == nil {
			return nil
		}
		w, err := c.writeFileHeader(profileName)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, cpuProfile)
		return err
	}()

	if traceErr != nil {
		return traceErr
	}
	return profileErr
}

func (c *Collector) addTimeBasedProfile(ctx context.Context, name string, targetSize int64,
	start func(w io.Writer) error, stop func()) error {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pr, pw := io.Pipe()

	err := start(pw)
	if err != nil {
		// A profile is already in progress, such as by an interactive request
		// to /debug/pprof/{profile,trace}
		//
		// Skip this part of the debug bundle collection.
		return nil
	}

	// Now that we know we'll have data, prepare to add it to the profile
	// bundle.
	w, err := c.writeFileHeader(name)
	if err != nil {
		return err
	}

	if targetSize > 0 {
		w = &limitTriggerWriter{
			wr:        w,
			fn:        cancel,
			remaining: targetSize,
		}
	}

	var copyErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, copyErr = io.Copy(w, pr)
	}()

	<-ctx.Done()
	stop()

	closeErr := pw.Close()
	wg.Wait()
	err = copyErr
	if err == nil {
		err = closeErr
	}

	return err
}

type limitTriggerWriter struct {
	wr        io.Writer
	fn        func()
	remaining int64
}

func (lw *limitTriggerWriter) Write(p []byte) (int, error) {
	n, err := lw.wr.Write(p)
	lw.remaining -= int64(n)
	if lw.remaining <= 0 && lw.fn != nil {
		lw.fn()
		lw.fn = nil
	}
	return n, err
}
