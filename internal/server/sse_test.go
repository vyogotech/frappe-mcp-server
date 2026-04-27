package server

import (
	"errors"
	"net/http"
	"testing"
)

// failingResponseWriter always returns an error from Write to simulate a
// disconnected client.
type failingResponseWriter struct {
	header http.Header
}

func (f *failingResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}
	return f.header
}
func (*failingResponseWriter) Write([]byte) (int, error) { return 0, errors.New("client disconnected") }
func (*failingResponseWriter) WriteHeader(int)            {}

type noopFlusher struct{}

func (noopFlusher) Flush() {}

func TestEmit_LogsWriteError(t *testing.T) {
	// Should not panic, should not block; just log and return.
	sw := &sseWriter{w: &failingResponseWriter{}, f: noopFlusher{}}
	sw.emit(sseEvent{Type: "content", Text: "x"})
}
