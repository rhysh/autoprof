package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/rhysh/autoprof"
)

func main() {
	ctx := context.Background()

	meta := autoprof.CurrentArchiveMeta()
	opt := &autoprof.ArchiveOptions{}
	buf := new(bytes.Buffer)

	err := autoprof.NewZipCollector(buf, meta, opt).Run(ctx)
	if err != nil {
		panic(err)
	} else {
		fmt.Printf("%s", buf.Bytes())
	}
}
