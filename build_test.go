package autoprof_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os/exec"
	"strings"
	"testing"

	"github.com/rhysh/autoprof"
)

func TestCurrentArchiveMeta(t *testing.T) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "go", "run", "./internal/test")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Errorf("go run: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr:\n%s", stderr.Bytes())
	}

	zr, err := zip.NewReader(bytes.NewReader(stdout.Bytes()), int64(stdout.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	mbuf, err := fs.ReadFile(zr, "meta")
	if err != nil {
		t.Fatalf("ReadFile(\"meta\"): %v", err)
	}

	var meta autoprof.ArchiveMeta
	err = json.Unmarshal(mbuf, &meta)
	if err != nil {
		t.Fatalf("json.Unmarshal(\"meta\"): %v", err)
	}

	if !strings.HasSuffix(meta.Main, "/internal/test") {
		t.Errorf("meta.Main: incorrect package name %q", meta.Main)
	}
}
