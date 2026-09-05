package main

import (
	"net/http"
	"testing"
)

type flushWriter struct{ flushed bool }

func (w *flushWriter) Header() http.Header         { return http.Header{} }
func (w *flushWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *flushWriter) WriteHeader(int)             {}
func (w *flushWriter) Flush()                      { w.flushed = true }

func TestLoopbackListenAddress(t *testing.T) {
	for _, value := range []string{"127.0.0.1:6061", "[::1]:6061"} {
		if !loopbackListenAddress(value) {
			t.Fatalf("%s must be accepted", value)
		}
	}
	for _, value := range []string{":6061", "0.0.0.0:6061", "192.168.1.1:6061", "localhost:6061"} {
		if loopbackListenAddress(value) {
			t.Fatalf("%s must be rejected", value)
		}
	}
}

func TestStatusRecorderPreservesFlush(t *testing.T) {
	base := &flushWriter{}
	recorder := &statusRecorder{ResponseWriter: base}
	var _ http.Flusher = recorder
	recorder.Flush()
	if !base.flushed {
		t.Fatal("Flush was not forwarded")
	}
}
