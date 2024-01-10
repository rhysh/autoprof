package periodic

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
)

func compare(tb testing.TB, count int, a io.ReadWriter) int64 {
	writeHash, readHash := sha256.New(), sha256.New()
	both := io.MultiWriter(a, writeHash)
	dashes := strings.Repeat("-", count)
	var nn int64
	for i := 0; i < count; i++ {
		n, _ := fmt.Fprintf(both, "%s this is line %d\n", dashes[:i], i)
		nn += int64(n)
	}

	_, err := io.Copy(readHash, a)
	if err != nil {
		tb.Fatalf("hash copy error: %v", err)
	}

	if have, want := writeHash.Sum(nil), readHash.Sum(nil); !bytes.Equal(have, want) {
		tb.Errorf("hash mismatch: %02x != %02x", have, want)
	}

	return nn
}

func TestLinkedListBuffer(t *testing.T) {
	t.Run("bytes.Buffer", func(t *testing.T) { compare(t, 1000, new(bytes.Buffer)) })
	t.Run("linkedListBuffer-1", func(t *testing.T) { compare(t, 1000, &linkedListBuffer{Size: 1}) })
	t.Run("linkedListBuffer-10", func(t *testing.T) { compare(t, 1000, &linkedListBuffer{Size: 10}) })
	t.Run("linkedListBuffer-100", func(t *testing.T) { compare(t, 1000, &linkedListBuffer{Size: 100}) })
	t.Run("linkedListBuffer-1000", func(t *testing.T) { compare(t, 1000, &linkedListBuffer{Size: 1000}) })
	t.Run("linkedListBuffer-default", func(t *testing.T) { compare(t, 1000, &linkedListBuffer{}) })
}

func BenchmarkLinkedListBuffer(b *testing.B) {
	testcase := func(makeBuffer func() io.ReadWriter) func(b *testing.B) {
		return func(b *testing.B) {
			b.ReportAllocs()
			var nn int64
			for i := 0; i < b.N; i++ {
				nn = compare(b, 1000, makeBuffer())
			}
			b.SetBytes(nn)
		}
	}

	b.Run("bytes.Buffer", testcase(func() io.ReadWriter { return new(bytes.Buffer) }))
	b.Run("linkedListBuffer-1", testcase(func() io.ReadWriter { return &linkedListBuffer{Size: 1} }))
	b.Run("linkedListBuffer-10", testcase(func() io.ReadWriter { return &linkedListBuffer{Size: 10} }))
	b.Run("linkedListBuffer-100", testcase(func() io.ReadWriter { return &linkedListBuffer{Size: 100} }))
	b.Run("linkedListBuffer-1000", testcase(func() io.ReadWriter { return &linkedListBuffer{Size: 1000} }))
	b.Run("linkedListBuffer-10000", testcase(func() io.ReadWriter { return &linkedListBuffer{Size: 10000} }))
	b.Run("linkedListBuffer-100000", testcase(func() io.ReadWriter { return &linkedListBuffer{Size: 100000} }))
	b.Run("linkedListBuffer-default", testcase(func() io.ReadWriter { return &linkedListBuffer{} }))
}
