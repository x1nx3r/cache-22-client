package syncer

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/cache-22/cache-22-client/internal/api"
	"github.com/cache-22/cache-22-client/internal/store"
)

func FillRanges(client *api.Client, serial string, s *store.Sparse, ranges [][2]int64, workers int) error {
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan [2]int64, len(ranges))
	for _, r := range ranges {
		jobs <- r
	}
	close(jobs)

	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				mu.Lock()
				failed := firstErr != nil
				mu.Unlock()
				if failed {
					return
				}
				var buf bytes.Buffer
				if err := client.DownloadRange(serial, r[0], r[1]-r[0], &buf); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("range %d-%d: %w", r[0], r[1], err)
					}
					mu.Unlock()
					return
				}
				mu.Lock()
				err := s.WriteRange(r[0], buf.Bytes())
				if err != nil && firstErr == nil {
					firstErr = fmt.Errorf("range %d-%d: %w", r[0], r[1], err)
				}
				mu.Unlock()
				if err != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func WholeImage(size int64, segment int64) [][2]int64 {
	if segment <= 0 {
		segment = 32 << 20
	}
	var out [][2]int64
	for off := int64(0); off < size; off += segment {
		end := off + segment
		if end > size {
			end = size
		}
		out = append(out, [2]int64{off, end})
	}
	return out
}

func PreloadRanges(size int64, boot [][2]int64, class string) [][2]int64 {
	head := int64(0)
	switch class {
	case "mid":
		head = 256 << 20
	case "slow":
		head = size / 4
	}
	if head > size {
		head = size
	}
	seen := map[[2]int64]bool{}
	var out [][2]int64
	push := func(r [2]int64) {
		if r[0] >= r[1] || seen[r] {
			return
		}
		seen[r] = true
		out = append(out, r)
	}
	for _, r := range boot {
		end := r[1]
		if end > size {
			end = size
		}
		push([2]int64{r[0], end})
	}
	if head > 0 {
		push([2]int64{0, head})
	}
	return out
}
