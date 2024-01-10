package autoprof

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseWaitDuration(t *testing.T) {
	testcase := func(s string, d time.Duration) func(t *testing.T) {
		return func(t *testing.T) {
			if have, want := parseWaitDuration(s), d; have != want {
				t.Errorf("parseWaitDuration(%q); %s != %s", s, have, want)
			}
		}
	}

	t.Run("", testcase("", 0))
	t.Run("", testcase("1", 0))
	t.Run("", testcase("one", 0))
	t.Run("", testcase("purple", 0))
	t.Run("", testcase("                ", 0))
	t.Run("", testcase("11111111", 0))
	t.Run("", testcase("1ss", 0))
	t.Run("", testcase("1ms", 0))
	t.Run("", testcase("1us", 0))
	t.Run("", testcase("1µs", 0))
	t.Run("", testcase("1ns", 0))
	t.Run("", testcase("1m", 0))
	t.Run("", testcase("1h", 0))

	t.Run("", testcase("1s", 1*time.Second))
	t.Run("", testcase("1.00s", 1*time.Second))
	t.Run("", testcase("1.0000000000000s", 1*time.Second))
	t.Run("", testcase("300.00s", 300*time.Second))
	t.Run("", testcase("3.000001s", 3*time.Second+1*time.Microsecond))

	t.Run("", testcase("-1s", 0))
	t.Run("", testcase(" 1s", 0))
	t.Run("", testcase("1s ", 0))
	t.Run("", testcase("1s 2s", 0))
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(&Handler{})
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http.Get; err = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(resp.Body); err = %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader; err = %v", err)
	}

	foundMeta := false
	for _, file := range zr.File {
		if file.Name == "meta" {
			foundMeta = true
		}
	}

	if !foundMeta {
		t.Errorf("profile bundle zip did not include 'meta' file")
	}
}
