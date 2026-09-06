package fusefs

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/x1nx3r/cache-22-client/internal/api"
	"github.com/x1nx3r/cache-22-client/internal/profile"
	"github.com/x1nx3r/cache-22-client/internal/store"
)

type FetchConfig struct {
	FetchSize     int64
	PrefetchDepth int64
}

func LANConfig() FetchConfig {
	return FetchConfig{FetchSize: 128 << 10, PrefetchDepth: 8}
}

func ConfigForRTT(ms float64) FetchConfig {
	switch {
	case ms < 5:
		return FetchConfig{FetchSize: 128 << 10, PrefetchDepth: 8}
	case ms < 50:
		return FetchConfig{FetchSize: 1 << 20, PrefetchDepth: 32}
	default:
		return FetchConfig{FetchSize: 2 << 20, PrefetchDepth: 64}
	}
}

type Reader struct {
	client *api.Client
	st     *store.Sparse
	size   int64
	serial string
	tracer *profile.Tracer
	cfg    FetchConfig

	mu       sync.Mutex
	inflight map[int64]chan struct{}
	fetchErr map[int64]error
	prevEnd  int64
}

func NewReader(client *api.Client, st *store.Sparse, size int64, serial string, cfg FetchConfig) *Reader {
	if cfg.FetchSize <= 0 {
		cfg = LANConfig()
	}
	return &Reader{client: client, st: st, size: size, serial: serial, cfg: cfg,
		inflight: map[int64]chan struct{}{}, fetchErr: map[int64]error{}}
}

func (r *Reader) Trace(t *profile.Tracer) {
	r.tracer = t
}

func (r *Reader) ensure(start, end int64) error {
	end = ((end-1)/r.cfg.FetchSize + 1) * r.cfg.FetchSize
	if end > r.size {
		end = r.size
	}
	r.mu.Lock()
	if ch, ok := r.inflight[start]; ok {
		r.mu.Unlock()
		<-ch
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.fetchErr[start]
	}
	ch := make(chan struct{})
	r.inflight[start] = ch
	r.mu.Unlock()

	err := r.fetch(start, end)

	r.mu.Lock()
	r.fetchErr[start] = err
	close(ch)
	delete(r.inflight, start)
	r.mu.Unlock()
	return err
}

func (r *Reader) fetch(start, end int64) error {
	var buf bytes.Buffer
	if err := r.client.DownloadRange(r.serial, start, end-start, &buf); err != nil {
		return fmt.Errorf("range %d-%d: %w", start, end, err)
	}
	return r.st.WriteRange(start, buf.Bytes())
}

func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, nil
	}
	if max := r.size - off; int64(len(p)) > max {
		p = p[:max]
	}
	r.tracer.Log(off, len(p))
	for _, m := range r.st.MissingRanges(off, int64(len(p))) {
		if err := r.ensure(m[0], m[1]); err != nil {
			return 0, err
		}
	}
	n, err := r.st.ReadAt(p, off)
	r.observe(off, int64(n))
	return n, err
}

func (r *Reader) observe(off, length int64) {
	end := off + length
	r.mu.Lock()
	contiguous := off == r.prevEnd
	r.prevEnd = end
	r.mu.Unlock()
	if !contiguous || length <= 0 {
		return
	}
	blockLen := r.st.BlockLen
	start := end
	stop := start + r.cfg.PrefetchDepth*blockLen
	if stop > r.size {
		stop = r.size
	}
	if start >= stop {
		return
	}
	go func() {
		for _, m := range r.st.MissingRanges(start, stop-start) {
			_ = r.ensure(m[0], m[1])
		}
	}()
}
