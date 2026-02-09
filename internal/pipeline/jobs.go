package pipeline

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/dgallion1/docgest/internal/doctree"
)

// JobStatus represents the state of an ingestion job.
type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusParsing    JobStatus = "parsing"
	StatusChunking   JobStatus = "chunking"
	StatusExtracting JobStatus = "extracting"
	StatusStoring    JobStatus = "storing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusPartial    JobStatus = "partial"
	StatusDupSkipped JobStatus = "duplicate_skipped"
)

// Job tracks the state of a single document ingestion.
type Job struct {
	mu sync.Mutex

	ID     string `json:"job_id"`
	DocID  string `json:"doc_id"`
	UserID string `json:"user_id"`

	Status   JobStatus `json:"status"`
	Phase    string    `json:"phase"`
	Filename string    `json:"filename"`
	Title    string    `json:"title"`

	Progress Progress `json:"progress"`

	ContentHash string    `json:"content_hash,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Internal: not serialized.
	fileData []byte
	chunks   []doctree.Chunk
	errors   []string
}

// Progress tracks processing progress.
type Progress struct {
	TotalChunks     int      `json:"total_chunks"`
	ChunksProcessed int      `json:"chunks_processed"`
	FactsValid      int      `json:"facts_valid"`
	FactsStored     int      `json:"facts_stored"`
	Errors          []string `json:"errors"`
}

// JobStore is a thread-safe in-memory job registry with TTL eviction.
type JobStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
	ttl  time.Duration
}

func NewJobStore(ttl time.Duration) *JobStore {
	return &JobStore{
		jobs: make(map[string]*Job),
		ttl:  ttl,
	}
}

func (s *JobStore) Put(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id]
}

// Cleanup removes expired jobs.
func (s *JobStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, job := range s.jobs {
		if now.Sub(job.UpdatedAt) > s.ttl {
			delete(s.jobs, id)
		}
	}
}

// SetStatus updates job status atomically.
func (j *Job) SetStatus(status JobStatus, phase string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	j.Phase = phase
	j.UpdatedAt = time.Now()
}

// AddError records an error.
func (j *Job) AddError(err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.errors = append(j.errors, err)
	j.Progress.Errors = j.errors
	j.UpdatedAt = time.Now()
}

// IncrChunksProcessed atomically increments chunks processed.
func (j *Job) IncrChunksProcessed() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress.ChunksProcessed++
	j.UpdatedAt = time.Now()
}

// AddFacts records extracted/stored fact counts.
func (j *Job) AddFacts(valid, stored int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress.FactsValid += valid
	j.Progress.FactsStored += stored
	j.UpdatedAt = time.Now()
}

// SetTotalChunks records total chunk count.
func (j *Job) SetTotalChunks(n int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress.TotalChunks = n
	j.UpdatedAt = time.Now()
}

// SetFileData sets the raw file bytes for processing.
func (j *Job) SetFileData(data []byte) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.fileData = data
}

// FileData returns the raw file bytes.
func (j *Job) FileData() []byte {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.fileData
}

// JobSnapshot is a read-only, JSON-safe copy of job state.
type JobSnapshot struct {
	ID       string    `json:"job_id"`
	DocID    string    `json:"doc_id"`
	UserID   string    `json:"user_id"`
	Status   JobStatus `json:"status"`
	Phase    string    `json:"phase"`
	Filename string    `json:"filename"`
	Title    string    `json:"title"`
	Progress Progress  `json:"progress"`
}

// Snapshot returns a JSON-safe copy of the job state.
func (j *Job) Snapshot() JobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	errs := j.Progress.Errors
	if errs == nil {
		errs = []string{}
	}
	return JobSnapshot{
		ID:       j.ID,
		DocID:    j.DocID,
		UserID:   j.UserID,
		Status:   j.Status,
		Phase:    j.Phase,
		Filename: j.Filename,
		Title:    j.Title,
		Progress: Progress{
			TotalChunks:     j.Progress.TotalChunks,
			ChunksProcessed: j.Progress.ChunksProcessed,
			FactsValid:      j.Progress.FactsValid,
			FactsStored:     j.Progress.FactsStored,
			Errors:          errs,
		},
	}
}

// ContentHashHex computes SHA-256 of content and returns hex string.
func ContentHashHex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:])
}
