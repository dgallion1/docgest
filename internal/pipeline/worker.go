package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dgallion1/docgest/internal/chunker"
	"github.com/dgallion1/docgest/internal/doctree"
	"github.com/dgallion1/docgest/internal/extract"
	"github.com/dgallion1/docgest/internal/parser"
	"github.com/dgallion1/docgest/internal/pathstore"
)

// Worker processes a single document job.
type Worker struct {
	claude    *extract.ClaudeClient
	pathstore *pathstore.Client
	log       *slog.Logger
	chunkCfg  chunker.Config

	maxConcurrentExtract int
	maxConcurrentStore   int
}

func NewWorker(claude *extract.ClaudeClient, ps *pathstore.Client, log *slog.Logger, chunkCfg chunker.Config, maxExtract, maxStore int) *Worker {
	return &Worker{
		claude:               claude,
		pathstore:            ps,
		log:                  log,
		chunkCfg:             chunkCfg,
		maxConcurrentExtract: maxExtract,
		maxConcurrentStore:   maxStore,
	}
}

// Process runs the full ingest pipeline for a job.
func (w *Worker) Process(ctx context.Context, job *Job) {
	log := w.log.With("job_id", job.ID, "doc_id", job.DocID, "user_id", job.UserID)

	// Phase 1: Parse
	job.SetStatus(StatusParsing, "parsing")
	p, err := parser.ForFile(job.Filename)
	if err != nil {
		log.Error("unsupported format", "error", err)
		job.AddError(err.Error())
		job.SetStatus(StatusFailed, "parsing")
		return
	}

	tree, err := p.Parse(bytes.NewReader(job.fileData), job.Filename)
	if err != nil {
		log.Error("parse failed", "error", err)
		job.AddError(fmt.Sprintf("parse: %s", err))
		job.SetStatus(StatusFailed, "parsing")
		return
	}
	if job.Title != "" {
		tree.Title = job.Title
	}

	// Compute content hash from the parsed text.
	parsedText := flattenTreeText(tree)
	job.ContentHash = ContentHashHex([]byte(parsedText))

	// Phase 1.5: Dedup check
	exists, existingDocID, err := w.checkDuplicate(ctx, job)
	if err != nil {
		log.Warn("dedup check failed, proceeding", "error", err)
	} else if exists {
		log.Info("duplicate document, skipping", "existing_doc_id", existingDocID)
		job.SetStatus(StatusDupSkipped, "dedup")
		return
	}

	// Phase 2: Chunk
	job.SetStatus(StatusChunking, "chunking")
	chunks := chunker.ChunkTree(tree, w.chunkCfg)
	job.SetTotalChunks(len(chunks))
	log.Info("chunked document", "chunks", len(chunks))

	if len(chunks) == 0 {
		log.Warn("no chunks produced")
		job.AddError("no extractable content")
		job.SetStatus(StatusFailed, "chunking")
		return
	}

	// Phase 3: Extract facts from chunks with bounded concurrency.
	job.SetStatus(StatusExtracting, "extracting")
	type chunkResult struct {
		facts []extract.Fact
		err   error
		idx   int
	}
	results := make(chan chunkResult, len(chunks))
	sem := make(chan struct{}, w.maxConcurrentExtract)

	for i, chunk := range chunks {
		sem <- struct{}{}
		go func(i int, chunk chunker.ChunkInput) {
			defer func() { <-sem }()
			prompt := extract.BuildChunkPrompt(tree.Title, chunk.Breadcrumb, chunk.Text)
			var facts []extract.Fact
			var lastErr error
			for attempt := range MaxRetries {
				facts, lastErr = w.claude.ExtractFacts(ctx, prompt)
				if lastErr == nil || !IsRetryable(lastErr) {
					break
				}
				log.Warn("retryable extraction error", "chunk", i, "attempt", attempt, "error", lastErr)
				select {
				case <-time.After(Backoff(attempt)):
				case <-ctx.Done():
					results <- chunkResult{err: ctx.Err(), idx: i}
					return
				}
			}
			results <- chunkResult{facts: facts, err: lastErr, idx: i}
		}(i, chunker.ChunkInput{Text: chunk.Text, Breadcrumb: chunk.Breadcrumb})
	}

	// Collect extraction results.
	var allFacts []extract.Fact
	hadErrors := false
	for range chunks {
		r := <-results
		job.IncrChunksProcessed()
		if r.err != nil {
			log.Error("extraction failed", "chunk", r.idx, "error", r.err)
			job.AddError(fmt.Sprintf("chunk %d: %s", r.idx, r.err))
			hadErrors = true
			continue
		}
		for i := range r.facts {
			if extract.ValidateFact(&r.facts[i]) {
				allFacts = append(allFacts, r.facts[i])
			}
		}
	}

	job.AddFacts(len(allFacts), 0)
	log.Info("extraction complete", "valid_facts", len(allFacts), "errors", hadErrors)

	if len(allFacts) == 0 && hadErrors {
		job.SetStatus(StatusFailed, "extracting")
		return
	}

	// Phase 4: Store facts in pathstore.
	job.SetStatus(StatusStoring, "storing")
	prefix := fmt.Sprintf("memory/users/%s", job.UserID)
	docPrefix := fmt.Sprintf("memory/users/%s/documents/%s", job.UserID, job.DocID)
	storedCount := 0

	storeSem := make(chan struct{}, w.maxConcurrentStore)
	type storeResult struct {
		ok   bool
		err  error
		path string
	}
	storeResults := make(chan storeResult, len(allFacts))

	for _, fact := range allFacts {
		storeSem <- struct{}{}
		go func(f extract.Fact) {
			defer func() { <-storeSem }()
			factPath, err := w.storeFact(ctx, f, prefix, job.DocID)
			if err != nil {
				storeResults <- storeResult{ok: false, err: err, path: factPath}
				return
			}
			// Write manifest entry.
			manifestPath := fmt.Sprintf("%s/facts/%s", docPrefix, extractULID(factPath))
			manifestErr := w.pathstore.PutNode(ctx, manifestPath, pathstore.NodeRequest{
				Value: map[string]any{
					"path":     factPath,
					"category": f.Category,
				},
				MemoryType: "metacognitive",
				Salience:   0.1,
				Source:      "docgest:" + job.DocID,
			})
			if manifestErr != nil {
				log.Warn("manifest write failed", "path", manifestPath, "error", manifestErr)
			}
			storeResults <- storeResult{ok: true, path: factPath}
		}(fact)
	}

	for range allFacts {
		r := <-storeResults
		if r.ok {
			storedCount++
		} else {
			log.Error("store failed", "path", r.path, "error", r.err)
			job.AddError(fmt.Sprintf("store %s: %s", r.path, r.err))
			hadErrors = true
		}
	}

	job.AddFacts(0, storedCount)
	log.Info("storage complete", "stored", storedCount, "total", len(allFacts))

	// Write document metadata.
	metaErr := w.pathstore.PutNode(ctx, docPrefix+"/meta", pathstore.NodeRequest{
		Value: map[string]any{
			"filename":     job.Filename,
			"title":        tree.Title,
			"content_hash": job.ContentHash,
			"facts_stored": storedCount,
			"total_chunks": len(chunks),
			"created_at":   job.CreatedAt.Format(time.RFC3339),
		},
		MemoryType: "metacognitive",
		Salience:   0.5,
		Source:     "docgest:" + job.DocID,
	})
	if metaErr != nil {
		log.Error("meta write failed", "error", metaErr)
		job.AddError(fmt.Sprintf("meta: %s", metaErr))
	}

	// Write hash index for dedup.
	hashPath := fmt.Sprintf("memory/users/%s/documents/by_hash/%s/%s", job.UserID, job.ContentHash, job.DocID)
	hashErr := w.pathstore.PutNode(ctx, hashPath, pathstore.NodeRequest{
		Value: map[string]any{
			"filename":   job.Filename,
			"created_at": job.CreatedAt.Format(time.RFC3339),
		},
		MemoryType: "metacognitive",
		Salience:   0.1,
		Source:     "docgest:" + job.DocID,
	})
	if hashErr != nil {
		log.Error("hash index write failed", "error", hashErr)
	}

	if hadErrors && storedCount > 0 {
		job.SetStatus(StatusPartial, "done")
	} else if hadErrors {
		job.SetStatus(StatusFailed, "storing")
	} else {
		job.SetStatus(StatusCompleted, "done")
	}
}

// storeFact writes a single fact to pathstore and returns the path used.
func (w *Worker) storeFact(ctx context.Context, f extract.Fact, prefix, docID string) (string, error) {
	info, ok := extract.CategoryMap[f.Category]
	if !ok {
		return "", fmt.Errorf("unknown category: %s", f.Category)
	}

	entity := extract.Slugify(f.Entity)
	if entity == "" {
		entity = "general"
	}
	topics := make([]string, 0, len(f.Topics))
	for _, t := range f.Topics {
		s := extract.Slugify(t)
		if s != "" {
			topics = append(topics, s)
		}
	}

	ulid := generateULID()
	var path string
	switch f.Category {
	case "entity_fact", "preference":
		tmpl := strings.Replace(info.PathTemplate, "{entity}", entity, 1)
		path = fmt.Sprintf("%s/%s/%s", prefix, tmpl, ulid)
	case "topic_knowledge", "procedure":
		topic := "general"
		if len(topics) > 0 {
			topic = topics[0]
		}
		tmpl := strings.Replace(info.PathTemplate, "{topic}", topic, 1)
		path = fmt.Sprintf("%s/%s/%s", prefix, tmpl, ulid)
	default:
		return "", fmt.Errorf("unexpected category: %s", f.Category)
	}

	salience := f.Salience
	if salience == 0 {
		salience = info.DefaultSal
	}

	value := map[string]any{
		"text":      f.Text,
		"entity":    f.Entity,
		"topics":    topics,
		"min_trust": f.MinTrust,
		"source": map[string]any{
			"type":   "document",
			"doc_id": docID,
		},
	}

	err := w.pathstore.PutNode(ctx, path, pathstore.NodeRequest{
		Value:      value,
		MemoryType: info.MemoryType,
		Salience:   salience,
		Source:     "docgest:" + docID,
	})
	return path, err
}

// checkDuplicate checks if this content hash already exists for the user.
func (w *Worker) checkDuplicate(ctx context.Context, job *Job) (bool, string, error) {
	hashPrefix := fmt.Sprintf("memory/users/%s/documents/by_hash/%s", job.UserID, job.ContentHash)
	children, err := w.pathstore.ListChildren(ctx, hashPrefix, 1)
	if err != nil {
		return false, "", err
	}
	if len(children) > 0 {
		// Extract doc_id from the key path.
		parts := strings.Split(children[0].Key, ".")
		docID := parts[len(parts)-1]
		return true, docID, nil
	}
	return false, "", nil
}

// flattenTreeText extracts all text from a DocTree into a single string for hashing.
func flattenTreeText(tree *doctree.DocTree) string {
	// Use a DoctTree import to avoid confusion.
	var sb strings.Builder
	var walk func(nodes []*doctree.DocNode)
	walk = func(nodes []*doctree.DocNode) {
		for _, n := range nodes {
			if n.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(n.Text)
			}
			walk(n.Children)
		}
	}
	walk(tree.Children)
	return sb.String()
}

// extractULID gets the last path segment (the ULID) from a full path.
func extractULID(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}
