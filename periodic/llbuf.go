package periodic

import "io"

type linkedListBuffer struct {
	Size int

	// head is the first item in the linked list.
	//
	// invariant: bufferLink values in the list have non-zero body length.
	head *bufferLink
	tail *bufferLink
}

type bufferLink struct {
	next *bufferLink
	// body stores the contents of this link.
	//
	// invariant: the capacity of body is not 0.
	body []byte
}

func (b *linkedListBuffer) newLink() *bufferLink {
	size := b.Size
	const defaultSize = 16 << 10
	if size <= 0 {
		size = defaultSize
	}
	return &bufferLink{body: make([]byte, 0, size)}
}

func (b *linkedListBuffer) Write(p []byte) (int, error) {
	l := len(p)
	if b.head == nil && l > 0 { // don't insert zero-length body
		b.tail = b.newLink()
		b.head = b.tail
	}
	for len(p) > 0 {
		start := len(b.tail.body)
		dst := b.tail.body[start:cap(b.tail.body)]
		n := copy(dst, p)
		b.tail.body = b.tail.body[:start+n]
		p = p[n:]

		if n == 0 {
			b.tail.next = b.newLink()
			b.tail = b.tail.next
		}
	}
	return l, nil
}

func (b *linkedListBuffer) Read(p []byte) (int, error) {
	if b.head == nil {
		return 0, io.EOF
	}

	n := copy(p, b.head.body)
	b.head.body = b.head.body[n:]

	if len(b.head.body) == 0 {
		b.head = b.head.next
	}

	return n, nil
}
