package logbuffer

import (
	"sync"
	"time"
)

const defaultMaxLines = 500

// Buffer holds the last N log lines in memory for admin display.
type Buffer struct {
	mu    sync.RWMutex
	lines []string
	max   int
}

func New(maxLines int) *Buffer {
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	return &Buffer{lines: make([]string, 0, maxLines), max: maxLines}
}

// Append adds a line with timestamp prefix.
func (b *Buffer) Append(line string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, ts+" "+line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
}

// Lines returns a copy of the buffer (newest last).
func (b *Buffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}
