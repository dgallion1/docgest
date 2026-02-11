package extract

import (
	"testing"
	"time"
)

func TestLLMStatsSnapshotPercentiles(t *testing.T) {
	stats := NewLLMStats(time.Hour)
	stats.Record(100)
	stats.Record(200)
	stats.Record(300)
	stats.Record(400)
	stats.Record(500)

	snap := stats.Snapshot()
	if snap.Count != 5 {
		t.Fatalf("expected count=5, got %d", snap.Count)
	}
	if snap.MinMs != 100 {
		t.Fatalf("expected min=100, got %d", snap.MinMs)
	}
	if snap.MaxMs != 500 {
		t.Fatalf("expected max=500, got %d", snap.MaxMs)
	}
	if snap.AvgMs != 300 {
		t.Fatalf("expected avg=300, got %f", snap.AvgMs)
	}
	if snap.P50Ms != 300 {
		t.Fatalf("expected p50=300, got %f", snap.P50Ms)
	}
	if snap.P95Ms != 480 {
		t.Fatalf("expected p95=480, got %f", snap.P95Ms)
	}
	if snap.P99Ms != 496 {
		t.Fatalf("expected p99=496, got %f", snap.P99Ms)
	}
}

func TestLLMStatsPrunesExpiredSamples(t *testing.T) {
	stats := NewLLMStats(10 * time.Millisecond)
	stats.Record(100)
	time.Sleep(25 * time.Millisecond)

	snap := stats.Snapshot()
	if snap.Count != 0 {
		t.Fatalf("expected count=0 after prune, got %d", snap.Count)
	}

	stats.Record(200)
	snap = stats.Snapshot()
	if snap.Count != 1 {
		t.Fatalf("expected count=1 for fresh sample, got %d", snap.Count)
	}
	if snap.MinMs != 200 || snap.MaxMs != 200 {
		t.Fatalf("expected min=max=200, got min=%d max=%d", snap.MinMs, snap.MaxMs)
	}
}

func TestLLMStatsRecordClampsNegativeDuration(t *testing.T) {
	stats := NewLLMStats(time.Hour)
	stats.Record(-10)
	snap := stats.Snapshot()
	if snap.Count != 1 {
		t.Fatalf("expected count=1, got %d", snap.Count)
	}
	if snap.MinMs != 0 || snap.MaxMs != 0 {
		t.Fatalf("expected clamped duration=0, got min=%d max=%d", snap.MinMs, snap.MaxMs)
	}
}
