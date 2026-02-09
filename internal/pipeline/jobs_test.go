package pipeline

import (
	"testing"
	"time"
)

func TestContentHashHex_Consistency(t *testing.T) {
	data := []byte("hello world")
	h1 := ContentHashHex(data)
	h2 := ContentHashHex(data)
	if h1 != h2 {
		t.Errorf("expected identical hashes, got %q and %q", h1, h2)
	}
	// SHA-256 of "hello world" is well-known.
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if h1 != want {
		t.Errorf("expected hash %q, got %q", want, h1)
	}
}

func TestContentHashHex_DifferentInputs(t *testing.T) {
	h1 := ContentHashHex([]byte("aaa"))
	h2 := ContentHashHex([]byte("bbb"))
	if h1 == h2 {
		t.Error("expected different hashes for different inputs")
	}
}

func TestContentHashHex_EmptyInput(t *testing.T) {
	h := ContentHashHex([]byte{})
	// SHA-256 of empty input is well-known.
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if h != want {
		t.Errorf("expected hash %q, got %q", want, h)
	}
}

func TestJob_StateTransitions(t *testing.T) {
	job := &Job{
		ID:        "test-1",
		Status:    StatusQueued,
		Phase:     "queued",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	transitions := []struct {
		status JobStatus
		phase  string
	}{
		{StatusParsing, "parsing document"},
		{StatusChunking, "splitting into chunks"},
		{StatusExtracting, "extracting facts"},
		{StatusStoring, "storing results"},
		{StatusCompleted, "done"},
	}

	for _, tr := range transitions {
		before := job.UpdatedAt
		// Small sleep to ensure time difference is detectable.
		time.Sleep(time.Millisecond)
		job.SetStatus(tr.status, tr.phase)

		if job.Status != tr.status {
			t.Errorf("expected status %q, got %q", tr.status, job.Status)
		}
		if job.Phase != tr.phase {
			t.Errorf("expected phase %q, got %q", tr.phase, job.Phase)
		}
		if !job.UpdatedAt.After(before) {
			t.Errorf("expected UpdatedAt to advance after SetStatus(%q)", tr.status)
		}
	}
}

func TestJob_SetStatusFailed(t *testing.T) {
	job := &Job{
		ID:        "test-fail",
		Status:    StatusExtracting,
		UpdatedAt: time.Now(),
	}
	job.SetStatus(StatusFailed, "extraction error")
	if job.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, job.Status)
	}
}

func TestJob_AddError(t *testing.T) {
	job := &Job{ID: "err-test", UpdatedAt: time.Now()}
	job.AddError("chunk 3 failed")
	job.AddError("chunk 7 failed")

	snap := job.Snapshot()
	if len(snap.Progress.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(snap.Progress.Errors))
	}
	if snap.Progress.Errors[0] != "chunk 3 failed" {
		t.Errorf("expected first error %q, got %q", "chunk 3 failed", snap.Progress.Errors[0])
	}
}

func TestJob_IncrChunksProcessed(t *testing.T) {
	job := &Job{ID: "incr-test", UpdatedAt: time.Now()}
	job.IncrChunksProcessed()
	job.IncrChunksProcessed()
	job.IncrChunksProcessed()

	snap := job.Snapshot()
	if snap.Progress.ChunksProcessed != 3 {
		t.Errorf("expected 3 chunks processed, got %d", snap.Progress.ChunksProcessed)
	}
}

func TestJob_AddFacts(t *testing.T) {
	job := &Job{ID: "facts-test", UpdatedAt: time.Now()}
	job.AddFacts(5, 4)
	job.AddFacts(3, 3)

	snap := job.Snapshot()
	if snap.Progress.FactsValid != 8 {
		t.Errorf("expected 8 valid facts, got %d", snap.Progress.FactsValid)
	}
	if snap.Progress.FactsStored != 7 {
		t.Errorf("expected 7 stored facts, got %d", snap.Progress.FactsStored)
	}
}

func TestJob_SetTotalChunks(t *testing.T) {
	job := &Job{ID: "total-test", UpdatedAt: time.Now()}
	job.SetTotalChunks(42)

	snap := job.Snapshot()
	if snap.Progress.TotalChunks != 42 {
		t.Errorf("expected 42 total chunks, got %d", snap.Progress.TotalChunks)
	}
}

func TestJob_FileData(t *testing.T) {
	job := &Job{ID: "data-test"}
	data := []byte("file content here")
	job.SetFileData(data)
	got := job.FileData()
	if string(got) != string(data) {
		t.Errorf("expected file data %q, got %q", data, got)
	}
}

func TestJob_SnapshotErrorsNotNil(t *testing.T) {
	// Snapshot should always return non-nil errors slice.
	job := &Job{ID: "snap-test", UpdatedAt: time.Now()}
	snap := job.Snapshot()
	if snap.Progress.Errors == nil {
		t.Error("expected non-nil errors slice in snapshot")
	}
	if len(snap.Progress.Errors) != 0 {
		t.Errorf("expected empty errors, got %d", len(snap.Progress.Errors))
	}
}

func TestJobStore_PutGet(t *testing.T) {
	store := NewJobStore(time.Hour)
	job := &Job{ID: "store-1", UpdatedAt: time.Now()}
	store.Put(job)

	got := store.Get("store-1")
	if got == nil {
		t.Fatal("expected to get job back")
	}
	if got.ID != "store-1" {
		t.Errorf("expected ID %q, got %q", "store-1", got.ID)
	}
}

func TestJobStore_GetMissing(t *testing.T) {
	store := NewJobStore(time.Hour)
	if store.Get("nonexistent") != nil {
		t.Error("expected nil for missing job")
	}
}

func TestJobStore_TTLCleanup(t *testing.T) {
	store := NewJobStore(50 * time.Millisecond)

	expired := &Job{ID: "old", UpdatedAt: time.Now()}
	store.Put(expired)

	// Wait for the TTL to pass.
	time.Sleep(100 * time.Millisecond)

	// Add a fresh job.
	fresh := &Job{ID: "new", UpdatedAt: time.Now()}
	store.Put(fresh)

	store.Cleanup()

	if store.Get("old") != nil {
		t.Error("expected expired job to be cleaned up")
	}
	if store.Get("new") == nil {
		t.Error("expected fresh job to survive cleanup")
	}
}

func TestJobStore_CleanupEmpty(t *testing.T) {
	store := NewJobStore(time.Hour)
	// Should not panic on empty store.
	store.Cleanup()
}
