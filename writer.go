package autoprof

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"runtime/pprof"
)

func errSource(err error) *DataSource {
	return &DataSource{WriteTo: func(ctx context.Context, w io.Writer) error {
		return err
	}}
}

func metaSource(meta *ArchiveMeta) *DataSource {
	buf, err := json.Marshal(meta)
	if err != nil {
		return errSource(err)
	}

	return &DataSource{WriteTo: func(ctx context.Context, w io.Writer) error {
		_, err = w.Write(buf)
		if err != nil {
			return err
		}
		return nil
	}}
}

func expvarSource() *DataSource {
	return expvarStyleSource(expvar.Do)
}

func expvarStyleSource(do func(func(kv expvar.KeyValue))) *DataSource {
	return &DataSource{WriteTo: func(ctx context.Context, w io.Writer) error {
		var err error
		printf := func(format string, a ...interface{}) {
			if err == nil {
				_, err = fmt.Fprintf(w, format, a...)
			}
		}

		prefix := ""
		printf("{\n")
		do(func(kv expvar.KeyValue) {
			printf("%s%q: %s", prefix, kv.Key, kv.Value)
			prefix = ",\n"
		})
		printf("\n}\n")

		return nil
	}}
}

func pprofSource(profile *pprof.Profile) *DataSource {
	return &DataSource{WriteTo: func(ctx context.Context, w io.Writer) error {
		return profile.WriteTo(w, 0)
	}}
}
