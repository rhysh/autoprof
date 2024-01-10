package autoprof

import (
	"bytes"
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"sync/atomic"
	"testing"
)

func readAll(source *DataSource) ([]byte, error) {
	var buf bytes.Buffer
	err := source.WriteTo(context.Background(), &buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var unique int64

func TestExpvarWriter(t *testing.T) {
	t.Run("valid-json", func(t *testing.T) {
		buf, err := readAll(expvarSource())
		if err != nil {
			t.Fatalf("readAll; err = %v", err)
		}
		v := make(map[string]interface{})
		err = json.Unmarshal(buf, &v)
		if err != nil {
			t.Fatalf("json.Unmarshal; err = %v", err)
		}
		_, ok := v["cmdline"]
		if !ok {
			t.Errorf("cmdline value not present")
		}
	})

	t.Run("name", func(t *testing.T) {
		buf, err := readAll(expvarStyleSource(func(f func(expvar.KeyValue)) {
			v1 := expvar.NewString(fmt.Sprintf("var-%d", atomic.AddInt64(&unique, 1)))
			v1.Set("world")
			f(expvar.KeyValue{
				Key:   "hello",
				Value: v1,
			})
			v2 := expvar.NewString(fmt.Sprintf("var-%d", atomic.AddInt64(&unique, 1)))
			v2.Set("bar")
			f(expvar.KeyValue{
				Key:   "foo",
				Value: v2,
			})
		}))
		if err != nil {
			t.Fatalf("readAll; err = %v", err)
		}
		if have, want := string(buf), `
{
"hello": "world",
"foo": "bar"
}
`[1:]; have != want {
			t.Errorf("output:\n%q\n!=\n%q", have, want)
		}
	})
}
