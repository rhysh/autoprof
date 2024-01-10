package autoprof_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime/pprof"
	"runtime/trace"
	"testing"
	"time"

	"github.com/rhysh/autoprof"
)

func TestZipCollector(t *testing.T) {
	const (
		profileName            = "pprof/profile"
		traceName              = "pprof/trace"
		profileDuringTraceName = "pprof/profile-during-trace"
	)

	checkExist := func(t *testing.T, zr *zip.Reader, name string) {
		f, err := zr.Open(name)
		if err != nil {
			t.Errorf("zip.Reader.Open; err = %v", err)
			return
		}
		defer f.Close()
	}
	checkNotExist := func(t *testing.T, zr *zip.Reader, name string) {
		f, err := zr.Open(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return
			}
			t.Errorf("zip.Reader.Open; err = %v", err)
			return
		}
		t.Errorf("found file %q", name)
		defer f.Close()
	}

	ctx := context.Background()
	meta := autoprof.CurrentArchiveMeta()

	t.Run("basic", func(t *testing.T) {
		_, err := collect(ctx, meta, &autoprof.ArchiveOptions{})
		if err != nil {
			t.Fatalf("collect; err = %v", err)
		}
	})

	t.Run("profile only", func(t *testing.T) {
		if profileIsEnabled() {
			t.Skip("a CPU profile is already active")
		}
		defer func() {
			if profileIsEnabled() {
				t.Errorf("a CPU profile remained active")
			}
		}()

		// Bundles that request a CPU profile should include one
		zr, err := collect(ctx, meta, &autoprof.ArchiveOptions{
			CPUProfileDuration: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("collect; err = %v", err)
		}
		checkExist(t, zr, profileName)
		checkNotExist(t, zr, traceName)
		checkNotExist(t, zr, profileDuringTraceName)
	})

	t.Run("trace only", func(t *testing.T) {
		if trace.IsEnabled() {
			t.Skip("an execution trace is already active")
		}
		defer func() {
			if trace.IsEnabled() {
				t.Errorf("an execution trace remained active")
			}
		}()

		// Bundles that request an execution trace should include one
		zr, err := collect(ctx, meta, &autoprof.ArchiveOptions{
			ExecutionTraceDuration: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("collect; err = %v", err)
		}
		checkNotExist(t, zr, profileName)
		checkExist(t, zr, traceName)
		checkNotExist(t, zr, profileDuringTraceName)
	})

	t.Run("profile and trace", func(t *testing.T) {
		if profileIsEnabled() {
			t.Skip("a CPU profile is already active")
		}
		if trace.IsEnabled() {
			t.Skip("an execution trace is already active")
		}
		defer func() {
			if profileIsEnabled() {
				t.Errorf("a CPU profile remained active")
			}
			if trace.IsEnabled() {
				t.Errorf("an execution trace remained active")
			}
		}()

		// Bundles that request both a CPU profile and an execution trace should
		// include both, plus a second CPU profile covering the duration of the
		// execution trace.
		zr, err := collect(ctx, meta, &autoprof.ArchiveOptions{
			CPUProfileDuration:     100 * time.Millisecond,
			ExecutionTraceDuration: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("collect; err = %v", err)
		}
		checkExist(t, zr, profileName)
		checkExist(t, zr, traceName)
		checkExist(t, zr, profileDuringTraceName)
	})

	t.Run("trace but profile already running", func(t *testing.T) {
		if profileIsEnabled() {
			t.Skip("a CPU profile is already active")
		}
		if trace.IsEnabled() {
			t.Skip("an execution trace is already active")
		}
		defer func() {
			if profileIsEnabled() {
				t.Errorf("a CPU profile remained active")
			}
			if trace.IsEnabled() {
				t.Errorf("an execution trace remained active")
			}
		}()

		err := pprof.StartCPUProfile(io.Discard)
		defer func() {
			if !profileIsEnabled() {
				t.Errorf("something stopped our CPU profile")
			}
			pprof.StopCPUProfile()
		}()

		// If a CPU profile is already running, we should still be able to
		// collect most of the bundle: It won't be able to include a CPU
		// profile, and it won't be able to include a CPU profile covering the
		// execution trace. But the collection should not result in an error,
		// and it should still include the execution trace.
		//
		// Furthermore, collecting the bundle (including the attempts to start a
		// CPU profile) should not interrupt the running CPU profile.
		zr, err := collect(ctx, meta, &autoprof.ArchiveOptions{
			CPUProfileDuration:     100 * time.Millisecond,
			ExecutionTraceDuration: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("collect; err = %v", err)
		}
		checkNotExist(t, zr, profileName)
		checkExist(t, zr, traceName)
		checkNotExist(t, zr, profileDuringTraceName)
	})

	t.Run("trace already running", func(t *testing.T) {
		if trace.IsEnabled() {
			t.Skip("an execution trace is already active")
		}
		defer func() {
			if trace.IsEnabled() {
				t.Errorf("an execution trace remained active")
			}
		}()

		err := trace.Start(io.Discard)
		defer func() {
			if !trace.IsEnabled() {
				t.Errorf("something stopped our execution trace")
			}
			trace.Stop()
		}()

		// If an execution trace is already running, we should still be able to
		// collect most of the bundle: it won't be able to include an execution
		// trace, but it should not result in an error.
		//
		// Furthermore, collecting the bundle should not interrupt the running
		// execution trace.
		zr, err := collect(ctx, meta, &autoprof.ArchiveOptions{
			ExecutionTraceDuration: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("collect; err = %v", err)
		}
		checkNotExist(t, zr, profileName)
		checkNotExist(t, zr, traceName)
		checkNotExist(t, zr, profileDuringTraceName)
	})

}

func collect(ctx context.Context, meta *autoprof.ArchiveMeta, opt *autoprof.ArchiveOptions) (*zip.Reader, error) {
	var buf bytes.Buffer

	err := autoprof.NewZipCollector(&buf, meta, opt).Run(context.Background())
	if err != nil {
		return nil, fmt.Errorf("autoprof.NewZipCollector.Run: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return nil, fmt.Errorf("zip.NewReader: %w", err)
	}

	return zr, nil
}

// profileIsEnabled returns whether a CPU profile is currently running.
func profileIsEnabled() bool {
	err := pprof.StartCPUProfile(io.Discard)
	if err != nil {
		return true
	}
	pprof.StopCPUProfile()
	return false
}
