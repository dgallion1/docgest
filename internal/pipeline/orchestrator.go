package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dgallion1/docgest/internal/chunker"
	"github.com/dgallion1/docgest/internal/config"
	"github.com/dgallion1/docgest/internal/extract"
	"github.com/dgallion1/docgest/internal/pathstore"
)

// Orchestrator manages the document ingestion pipeline.
type Orchestrator struct {
	jobs     *JobStore
	queue    chan *Job
	claude   *extract.ClaudeClient
	ps       *pathstore.Client
	log      *slog.Logger
	cfg      config.Config
	chunkCfg chunker.Config

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewOrchestrator creates and starts the pipeline.
func NewOrchestrator(cfg config.Config, claude *extract.ClaudeClient, ps *pathstore.Client, log *slog.Logger) *Orchestrator {
	o := &Orchestrator{
		jobs:  NewJobStore(cfg.JobTTL),
		queue: make(chan *Job, cfg.MaxQueueSize),
		claude: claude,
		ps:     ps,
		log:    log,
		cfg:    cfg,
		chunkCfg: chunker.Config{
			ChunkSize:    cfg.DefaultChunkSize,
			ChunkOverlap: cfg.DefaultChunkOverlap,
			MinChunk:     100,
		},
	}
	return o
}

// Start launches worker goroutines.
func (o *Orchestrator) Start(ctx context.Context) {
	workerCtx, cancel := context.WithCancel(ctx)
	o.cancel = cancel

	for range o.cfg.WorkerCount {
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
			w := NewWorker(o.claude, o.ps, o.log, o.chunkCfg, o.cfg.MaxConcurrentExtract, o.cfg.MaxConcurrentStore)
			for {
				select {
				case <-workerCtx.Done():
					return
				case job, ok := <-o.queue:
					if !ok {
						return
					}
					w.Process(workerCtx, job)
				}
			}
		}()
	}

	// Start job store cleanup.
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				o.jobs.Cleanup()
			}
		}
	}()
}

// Stop gracefully shuts down the pipeline.
func (o *Orchestrator) Stop() {
	if o.cancel != nil {
		o.cancel()
	}
	close(o.queue)
	o.wg.Wait()
}

// Submit queues a new job for processing.
func (o *Orchestrator) Submit(job *Job) error {
	o.jobs.Put(job)
	select {
	case o.queue <- job:
		return nil
	default:
		job.SetStatus(StatusFailed, "queue_full")
		return fmt.Errorf("job queue is full (%d)", o.cfg.MaxQueueSize)
	}
}

// GetJob returns a job by ID.
func (o *Orchestrator) GetJob(id string) *Job {
	return o.jobs.Get(id)
}

// QueueDepth returns current queue depth.
func (o *Orchestrator) QueueDepth() int {
	return len(o.queue)
}

// PathstoreClient returns the pathstore client for direct use by API handlers.
func (o *Orchestrator) PathstoreClient() *pathstore.Client {
	return o.ps
}
