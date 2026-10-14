package media

import (
	"io"
	"sync"
)

type RingBuffer[T any] struct {
	mu       sync.Mutex
	cond     *sync.Cond
	data     []T
	capacity int
	head     int
	tail     int
	size     int
	closed   bool
}

func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	r := &RingBuffer[T]{
		data:     make([]T, capacity),
		capacity: capacity,
	}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *RingBuffer[T]) WriteChunk(items []T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	n := len(items)
	if n == 0 {
		return
	}

	if n > r.capacity {
		items = items[n-r.capacity:]
		n = r.capacity
	}

	spaceToEnd := r.capacity - r.head
	if n <= spaceToEnd {
		copy(r.data[r.head:], items)
	} else {
		copy(r.data[r.head:], items[:spaceToEnd])
		copy(r.data[0:], items[spaceToEnd:])
	}

	r.head = (r.head + n) % r.capacity

	if r.size+n <= r.capacity {
		r.size += n
	} else {
		r.size = r.capacity
		r.tail = r.head
	}

	r.cond.Broadcast()
}

func (r *RingBuffer[T]) ReadChunk(chunk []T) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for r.size == 0 {
		if r.closed {
			return 0, io.EOF
		}
		r.cond.Wait()
	}

	readCount := len(chunk)
	if r.size < readCount {
		readCount = r.size
	}

	if r.tail+readCount <= r.capacity {
		copy(chunk, r.data[r.tail:r.tail+readCount])
	} else {
		firstPart := r.capacity - r.tail
		secondPart := readCount - firstPart
		copy(chunk[:firstPart], r.data[r.tail:r.capacity])
		copy(chunk[firstPart:], r.data[:secondPart])
	}

	r.tail = (r.tail + readCount) % r.capacity
	r.size -= readCount

	return readCount, nil
}

func (r *RingBuffer[T]) ReadExact(chunk []T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	target := len(chunk)
	for r.size < target {
		if r.closed {
			return io.EOF
		}
		r.cond.Wait()
	}

	if r.tail+target <= r.capacity {
		copy(chunk, r.data[r.tail:r.tail+target])
	} else {
		firstPart := r.capacity - r.tail
		secondPart := target - firstPart
		copy(chunk[:firstPart], r.data[r.tail:r.capacity])
		copy(chunk[firstPart:], r.data[:secondPart])
	}

	r.tail = (r.tail + target) % r.capacity
	r.size -= target

	return nil
}

func (r *RingBuffer[T]) DiscardChunk(n int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size < n {
		r.tail = r.head
		r.size = 0
		return nil
	}

	r.tail = (r.tail + n) % r.capacity
	r.size -= n
	return nil
}

func (r *RingBuffer[T]) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.cond.Broadcast()
}

func (r *RingBuffer[T]) IsClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}
