package tui

import "sync"

const DefaultBufferSize = 10000

type RingBuffer struct {
	mu    sync.Mutex
	buf   []string
	cap   int
	head  int
	count int
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]string, capacity),
		cap: capacity,
	}
}

// Write appends a line to the buffer, dropping the oldest line if full.
func (rb *RingBuffer) Write(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buf[rb.head] = line
	rb.head = (rb.head + 1) % rb.cap

	if rb.count < rb.cap {
		rb.count++
	}
}

// Lines returns all lines in the buffer in order (oldest to newest).
func (rb *RingBuffer) Lines() []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	result := make([]string, rb.count)

	for i := 0; i < rb.count; i++ {
		idx := (rb.head - rb.count + i) % rb.cap
		if idx < 0 {
			idx += rb.cap
		}
		result[i] = rb.buf[idx]
	}

	return result
}

// Clear resets the buffer to empty.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buf = make([]string, rb.cap)
	rb.head = 0
	rb.count = 0
}

// Len returns the current number of lines in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	return rb.count
}
