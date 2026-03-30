package log

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// stream holds the state for a single active log stream.
type stream struct {
	cancel context.CancelFunc
	ch     chan messages.LogLine
}

// StreamManager tracks and controls active log-reading goroutines.
type StreamManager struct {
	mu      sync.Mutex
	parent  context.Context
	streams map[string]*stream
}

// NewStreamManager creates a StreamManager whose streams are derived
// from the given parent context. Cancelling the parent will stop all
// streams.
func NewStreamManager(parent context.Context) *StreamManager {
	return &StreamManager{
		parent:  parent,
		streams: make(map[string]*stream),
	}
}

// StartStream begins reading from reader line-by-line in a background
// goroutine. Each line is parsed into a messages.LogLine and sent on the
// returned channel. The channel is closed when the reader is exhausted
// or the stream is stopped.
//
// If a stream with the same id already exists it is stopped first.
func (sm *StreamManager) StartStream(id string, reader io.ReadCloser, source string) <-chan messages.LogLine {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Stop any existing stream with this id.
	if existing, ok := sm.streams[id]; ok {
		existing.cancel()
		// Do not close the channel here; the goroutine owns it.
	}

	ctx, cancel := context.WithCancel(sm.parent) //nolint:gosec // cancel is stored in stream and called by StopStream
	ch := make(chan messages.LogLine, 128)

	s := &stream{
		cancel: cancel,
		ch:     ch,
	}
	sm.streams[id] = s

	go sm.readLoop(ctx, id, reader, source, ch)

	return ch
}

// readLoop reads lines from reader until EOF or context cancellation,
// parses each line, and sends it on ch. It closes ch and the reader
// when done.
func (sm *StreamManager) readLoop(ctx context.Context, id string, reader io.ReadCloser, source string, ch chan messages.LogLine) {
	defer close(ch)
	defer func() { _ = reader.Close() }()

	scanner := bufio.NewScanner(reader)
	// Allow lines up to 1 MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		text := scanner.Text()
		// Strip Docker multiplexed stream header and ANSI escape codes.
		text = stripDockerHeader(text)
		text = stripANSI(text)
		line := messages.LogLine{
			Source:   source,
			SourceID: id,
			Text:     text,
			Level:    ParseLogLevel(text),
			Time:     ParseTimestamp(text),
		}

		// If we could not extract a timestamp, use current time.
		if line.Time.IsZero() {
			line.Time = time.Now()
		}

		select {
		case ch <- line:
		case <-ctx.Done():
			return
		}
	}

	// Clean up from the map when done.
	sm.mu.Lock()
	delete(sm.streams, id)
	sm.mu.Unlock()
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape codes from a string.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// stripDockerHeader removes the 8-byte Docker multiplexed stream header
// that appears at the start of log lines from non-TTY containers.
// The header format is: [stream_type(1)][0][0][0][size(4)] = 8 bytes.
func stripDockerHeader(s string) string {
	if len(s) < 8 {
		return s
	}
	// Docker header starts with byte 0x01 (stdout) or 0x02 (stderr),
	// followed by three zero bytes.
	b := s[0]
	if (b == 0x01 || b == 0x02) && s[1] == 0x00 && s[2] == 0x00 && s[3] == 0x00 {
		return s[8:]
	}
	return s
}

// GetChannel returns the log channel for the given stream, or nil if not found.
func (sm *StreamManager) GetChannel(id string) <-chan messages.LogLine {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[id]; ok {
		return s.ch
	}
	return nil
}

// StopStream cancels and removes the stream identified by id.
// It is safe to call with an unknown id.
func (sm *StreamManager) StopStream(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[id]; ok {
		s.cancel()
		delete(sm.streams, id)
	}
}

// StopAll cancels and removes every active stream.
func (sm *StreamManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, s := range sm.streams {
		s.cancel()
		delete(sm.streams, id)
	}
}
