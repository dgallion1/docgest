package extract

import (
	"sort"
	"sync"
	"time"
)

type sample struct {
	timestamp  time.Time
	durationMs int64
}

// StatsSnapshot is a point-in-time aggregate of LLM latency samples.
type StatsSnapshot struct {
	Count int     `json:"count"`
	MinMs int64   `json:"min_ms"`
	MaxMs int64   `json:"max_ms"`
	AvgMs float64 `json:"avg_ms"`
	P50Ms float64 `json:"p50_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
}

// LLMStats tracks recent LLM call latencies within a rolling window.
type LLMStats struct {
	mu      sync.Mutex
	samples []sample
	maxAge  time.Duration
}

func NewLLMStats(maxAge time.Duration) *LLMStats {
	if maxAge <= 0 {
		maxAge = time.Hour
	}
	return &LLMStats{
		samples: make([]sample, 0, 256),
		maxAge:  maxAge,
	}
}

func (s *LLMStats) Record(durationMs int64) {
	if durationMs < 0 {
		durationMs = 0
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	s.samples = append(s.samples, sample{
		timestamp:  now,
		durationMs: durationMs,
	})
}

func (s *LLMStats) Snapshot() StatsSnapshot {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	if len(s.samples) == 0 {
		return StatsSnapshot{}
	}

	values := make([]int64, 0, len(s.samples))
	var sum int64
	for _, sm := range s.samples {
		values = append(values, sm.durationMs)
		sum += sm.durationMs
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })

	return StatsSnapshot{
		Count: len(values),
		MinMs: values[0],
		MaxMs: values[len(values)-1],
		AvgMs: float64(sum) / float64(len(values)),
		P50Ms: percentile(values, 50),
		P95Ms: percentile(values, 95),
		P99Ms: percentile(values, 99),
	}
}

func (s *LLMStats) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.maxAge)
	writeIdx := 0
	for _, sm := range s.samples {
		if !sm.timestamp.Before(cutoff) {
			s.samples[writeIdx] = sm
			writeIdx++
		}
	}
	s.samples = s.samples[:writeIdx]
}

func percentile(sortedValues []int64, pct float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if pct <= 0 {
		return float64(sortedValues[0])
	}
	if pct >= 100 {
		return float64(sortedValues[len(sortedValues)-1])
	}

	index := (float64(len(sortedValues)-1) * pct) / 100.0
	lower := int(index)
	upper := lower + 1
	if upper >= len(sortedValues) {
		return float64(sortedValues[lower])
	}
	if lower == upper {
		return float64(sortedValues[lower])
	}
	weight := index - float64(lower)
	lo := float64(sortedValues[lower])
	hi := float64(sortedValues[upper])
	return lo + ((hi - lo) * weight)
}
