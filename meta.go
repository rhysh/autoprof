package autoprof

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

const (
	rfc3339milli = "2006-01-02T15:04:05.000Z07:00"
)

func generateArchiveMeta() (*ArchiveMeta, error) {
	var reterr error

	initTime := time.Now().UTC()

	rng := rand.New(selectSource{})

	var buf [12]byte
	for i := range buf {
		buf[i] = byte(rng.Uint32())
	}
	procID := fmt.Sprintf("1-%08x-%02x-%d", uint32(initTime.Unix()), buf, os.Getpid())

	meta := &ArchiveMeta{
		Main:      "",
		Revision:  "",
		GoVersion: runtime.Version(),

		Hostname: "",
		ProcID:   procID,
		InitTime: initTime.Format(rfc3339milli),
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		meta.Main = info.Main.Path
		meta.Revision = info.Main.Version
	}

	hostname, err := os.Hostname()
	if err != nil {
		reterr = err
	}
	meta.Hostname = hostname

	return meta, reterr
}

var baseMeta *ArchiveMeta

func init() {
	baseMeta, _ = generateArchiveMeta()
}

// ArchiveMeta contains metadata about the current process, and about a
// profile bundle. In zip-archived profile bundles, this structure will be
// JSON-encoded and stored as a file named "meta".
type ArchiveMeta struct {
	Main      string `json:"main"`
	Revision  string `json:"revision"`
	GoVersion string `json:"go_version"`

	Hostname string `json:"hostname"`
	ProcID   string `json:"proc_id"`
	InitTime string `json:"init_time"`

	CaptureTime string `json:"capture_time"`
}

// CurrentArchiveMeta returns the ArchiveMeta value for a profile bundle
// scheduled to start immediately.
func CurrentArchiveMeta() *ArchiveMeta {
	now := time.Now().UTC()

	meta := *baseMeta
	meta.CaptureTime = now.Format(rfc3339milli)
	return &meta
}

// selectSource is a math/rand.Source PRNG based on the randomness in select
// statments. It uses the Go runtime's internal PRNG, which should be well-
// seeded. It is expected to provide decent entropy without the possibility of
// returning an error.
type selectSource struct{}

func (selectSource) Seed(_ int64) {}

func (selectSource) Int63() int64 {
	var v int64
	ch := make(chan struct{})
	close(ch)
	for i := uint(0); i < 63; i++ {
		select {
		case <-ch:
			v += 0 << i
		case <-ch:
			v += 1 << i
		}
	}
	return v
}
