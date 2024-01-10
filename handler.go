package autoprof

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Handler is an http.Handler which serves profile bundles.
//
// The HTTP caller can use the "profile" and "trace" query parameters to
// indicate the desired duration in seconds of a CPU profile and execution
// trace. A parameter send this way should be a positive floating point number
// with an "s" suffix, to indicate units of "seconds".
//
// This http.Handler should be mounted at "/debug/profiles".
type Handler struct {
}

var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	meta := CurrentArchiveMeta()

	opt := &ArchiveOptions{
		// Include output of /debug/pprof/profile
		CPUProfileDuration: parseWaitDuration(r.URL.Query().Get("profile")),
		// Include output of /debug/pprof/trace
		ExecutionTraceDuration: parseWaitDuration(r.URL.Query().Get("trace")),
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", downloadFileName(meta)))

	c := NewZipCollector(w, meta, opt)
	err := c.Run(r.Context())
	if err != nil {
		// make an effort to report .. but the headers have probably already
		// been sent
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func downloadFileName(meta *ArchiveMeta) string {
	return fmt.Sprintf("profile_%s_%s_%s.zip",
		url.PathEscape(path.Base(meta.Main)),
		url.PathEscape(meta.ProcID),
		url.PathEscape(meta.CaptureTime))
}

// parseWaitDuration returns a non-negative duration represented by the input
// s as a floating point number followed by an 's'. This matches the text
// (JSON) encoding of the google.protobuf.Duration type.
//
// It returns 0 if the input is invalid.
func parseWaitDuration(s string) time.Duration {
	seconds := strings.TrimSuffix(s, "s")
	v, err := strconv.ParseFloat(seconds, 64)
	if err != nil || v < 0 || s == seconds {
		return 0
	}
	return time.Duration(v * float64(time.Second))
}
