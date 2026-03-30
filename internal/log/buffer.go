package log

import (
	"sync"

	"github.com/abhishek-rana/lazydk/pkg/messages"
)

const defaultCapacity = 10000

// RingBuffer is a thread-safe ring buffer for log lines.
// When the buffer is full, the oldest entries are dropped.
type RingBuffer struct {
	mu       sync.RWMutex
	buf      []messages.LogLine
	head     int // index of the oldest element
	count    int
	capacity int
}

// NewRingBuffer creates a RingBuffer with the given capacity.
// If capacity is <= 0, the default of 10000 is used.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &RingBuffer{
		buf:      make([]messages.LogLine, capacity),
		capacity: capacity,
	}
}

// Write adds a log line to the buffer. If the buffer is at capacity,
// the oldest line is overwritten.
func (rb *RingBuffer) Write(line messages.LogLine) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	idx := (rb.head + rb.count) % rb.capacity
	rb.buf[idx] = line

	if rb.count == rb.capacity {
		// Buffer is full; advance head to drop the oldest entry.
		rb.head = (rb.head + 1) % rb.capacity
	} else {
		rb.count++
	}
}

// Lines returns a copy of all lines in the buffer, ordered oldest to newest.
func (rb *RingBuffer) Lines() []messages.LogLine {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	out := make([]messages.LogLine, rb.count)
	for i := 0; i < rb.count; i++ {
		out[i] = rb.buf[(rb.head+i)%rb.capacity]
	}
	return out
}

// Len returns the number of lines currently stored in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Clear empties the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.count = 0
}
