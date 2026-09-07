package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func patternBytes(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func TestDownloadRangeRetries(t *testing.T) {
	data := patternBytes(64 << 10)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "")
	var buf bytes.Buffer
	if err := c.DownloadRange("G", 0, int64(len(data)), &buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("retried download returned wrong bytes")
	}
	if hits.Load() != 2 {
		t.Errorf("hits = %d, want 2", hits.Load())
	}
}

func TestDownloadRangeNoRetryOnClientError(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "nope", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "")
	var buf bytes.Buffer
	if err := c.DownloadRange("G", 0, 100, &buf); err == nil {
		t.Fatal("want error")
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1 (403 must not retry)", hits.Load())
	}
}

func TestDownloadRangeResetsBufferBetweenAttempts(t *testing.T) {
	data := patternBytes(32 << 10)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(data[:1024])
			panic(http.ErrAbortHandler)
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "")
	var buf bytes.Buffer
	if err := c.DownloadRange("G", 0, int64(len(data)), &buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("buffer must contain exactly the retried body, not partial bytes from attempt 1")
	}
	if hits.Load() != 2 {
		t.Errorf("hits = %d, want 2", hits.Load())
	}
}
