package periodic

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"net/url"
	"time"

	"github.com/rhysh/autoprof"
)

const (
	// defaultProfileInterval adjusts the upper limit on the interval between
	// profiles: from the end of one to the start of the next. The package
	// applies jitter to the time between each profile, which will result in
	// slightly more frequent samples on average.
	defaultProfileInterval = 2 * time.Minute
)

// A Collector periodically builds a profile bundle for the process.
type Collector struct {
	StoreBundle func(meta *autoprof.ArchiveMeta, buf []byte)
}

// Run periodically builds a profile bundle for the processes and passes it to
// the provided StoreBundle function.
func (c *Collector) Run(ctx context.Context) error {
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// fatal error, return immediately
		return err
	}

	r := &runner{
		c:   c,
		rng: rand.New(rand.NewSource(seed.Int64())),
	}

	for i := 0; ; i++ {
		r.delay(ctx, i)
		err = ctx.Err()
		if err != nil {
			// fatal error, return immediately
			return err
		}

		opts := r.options(i)
		err := r.store(ctx, opts)
		if err != nil {
			return err
		}
	}
}

type runner struct {
	c *Collector

	rng           *rand.Rand
	nextExecTrace int
}

func (r *runner) options(i int) *autoprof.ArchiveOptions {
	var opts autoprof.ArchiveOptions

	opts.CPUProfileDuration = 5 * time.Second
	opts.CPUProfileByteTarget = 1e6

	// Execution traces are often quite large, and are harder to analyze in
	// aggregate in the same ways that pprof-formatted profiles can. Store fewer
	// of them.
	if r.nextExecTrace == i {
		const execTraceMaxPeriod = 100
		r.nextExecTrace = i + 1 + r.rng.Intn(execTraceMaxPeriod)
		opts.ExecutionTraceDuration = 1 * time.Second
	}
	opts.ExecutionTraceByteTarget = 1e7

	// Don't include variable-duration profiles on the first run; we'd like a
	// good chance of getting at least a little data from short-lived
	// processes.
	if i == 0 {
		opts.CPUProfileDuration = 0
		opts.ExecutionTraceDuration = 0
	}

	return &opts
}

func (r *runner) delay(ctx context.Context, i int) {
	max := int64(defaultProfileInterval)

	// shorten the delay by up to 100% on the first run, and by up to 20% on
	// subsequent runs
	maxTrim := max
	if i > 0 {
		maxTrim = max / 5
	}

	trim := r.rng.Int63n(maxTrim)
	t := time.NewTimer(time.Duration(max - trim))
	defer t.Stop()

	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (r *runner) store(ctx context.Context, opts *autoprof.ArchiveOptions) error {
	meta := autoprof.CurrentArchiveMeta()

	// Some profile types are sensitive to latency when writing out their data.
	// The execution tracer is one such profile type, which results in
	// systematic failure to record traces of garbage collection in progress in
	// some applications. Use a buffer type that does not introduce large
	// latency spikes.
	llb := &linkedListBuffer{}
	err := autoprof.NewZipCollector(llb, meta, opts).Run(ctx)
	if err != nil {
		return err
	}

	// Now that the latency-sensitive portion is complete, convert the buffer
	// into a format convenient for storage.
	buf, err := io.ReadAll(llb)
	if err != nil {
		return err
	}

	r.c.StoreBundle(meta, buf)

	return nil
}

func s3Key(m *autoprof.ArchiveMeta) string {
	return fmt.Sprintf("pprof/%s/%s/%s/%s",
		url.PathEscape(m.Main),
		url.PathEscape(m.Hostname),
		url.PathEscape(m.ProcID),
		url.PathEscape(m.CaptureTime))
}
