package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	for rtt, want := range map[float64]string{0: "lan", 4.9: "lan", 5: "mid", 49.9: "mid", 50: "slow", 500: "slow"} {
		if got := Classify(rtt); got != want {
			t.Errorf("Classify(%v) = %q, want %q", rtt, got, want)
		}
	}
}

func TestProbe(t *testing.T) {
	data := make([]byte, 10<<20)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := New(srv.URL, "").Probe("G")
	if err != nil {
		t.Fatal(err)
	}
	if res.Class != "lan" {
		t.Errorf("class = %q, want lan on loopback", res.Class)
	}
	if res.RTTMs <= 0 {
		t.Errorf("RTTMs = %v", res.RTTMs)
	}
	if res.Mbps <= 0 {
		t.Errorf("Mbps = %v", res.Mbps)
	}
}

func TestProbeDeadServer(t *testing.T) {
	if _, err := New("http://127.0.0.1:1", "").Probe("G"); err == nil {
		t.Error("want error, got nil")
	}
}
