//go:build darwin

package console

import (
	"bytes"
	"context"
	"testing"
)

func TestStreamStartsWithHistory(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	c.broadcast([]byte("boot log\n"))

	ctx, cancel := context.WithCancel(context.Background())
	var got bytes.Buffer
	err = c.Stream(ctx, cancelWriter{Buffer: &got, cancel: cancel})
	if err != context.Canceled {
		t.Fatalf("Stream() error = %v, want %v", err, context.Canceled)
	}
	if got.String() != "boot log\n" {
		t.Fatalf("Stream() output = %q, want %q", got.String(), "boot log\n")
	}
}

func TestHistoryKeepsTailWithinLimit(t *testing.T) {
	c := &Console{}
	c.appendHistory(bytes.Repeat([]byte("a"), logBufferSize))
	c.appendHistory([]byte("bc"))

	if len(c.history) != logBufferSize {
		t.Fatalf("history len = %d, want %d", len(c.history), logBufferSize)
	}
	if got := string(c.history[len(c.history)-2:]); got != "bc" {
		t.Fatalf("history tail = %q, want %q", got, "bc")
	}
}

type cancelWriter struct {
	*bytes.Buffer
	cancel context.CancelFunc
}

func (w cancelWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.cancel()
	return n, err
}
