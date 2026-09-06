package api

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const probeBytes = 8 << 20

type ProbeResult struct {
	RTTMs float64 `json:"rttMs"`
	Mbps  float64 `json:"mbps"`
	Class string  `json:"class"`
}

func Classify(rttMs float64) string {
	switch {
	case rttMs < 5:
		return "lan"
	case rttMs < 50:
		return "mid"
	default:
		return "slow"
	}
}

func (c *Client) Probe(serial string) (ProbeResult, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	var res ProbeResult

	rtts := make([]float64, 0, 5)
	for i := 0; i < 5; i++ {
		start := time.Now()
		req, err := http.NewRequest("GET", c.base+"/v1/health", nil)
		if err != nil {
			return res, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return res, fmt.Errorf("probe rtt: %w", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return res, fmt.Errorf("probe rtt: %s", resp.Status)
		}
		rtts = append(rtts, float64(time.Since(start).Microseconds())/1000)
	}
	sort.Float64s(rtts)
	res.RTTMs = rtts[len(rtts)/2]
	res.Class = Classify(res.RTTMs)

	req, err := http.NewRequest("GET", c.base+"/v1/files/"+serial, nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", probeBytes-1))
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return res, fmt.Errorf("probe throughput: %w", err)
	}
	n, err := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err != nil {
		return res, fmt.Errorf("probe throughput: %w", err)
	}
	secs := time.Since(start).Seconds()
	if secs <= 0 {
		secs = 0.001
	}
	res.Mbps = float64(n) * 8 / secs / 1e6
	return res, nil
}
