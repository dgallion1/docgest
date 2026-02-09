package chunker

import (
	"strings"
	"testing"

	"github.com/dgallion1/docgest/internal/doctree"
)

func TestChunkTree_SmallTreeFitsOneChunk(t *testing.T) {
	tree := &doctree.DocTree{
		Title: "Small",
		Children: []*doctree.DocNode{
			{
				Title: "Section",
				Text:  strings.Repeat("word ", 200), // ~200 words -> ~266 tokens, above default MinChunk
			},
		},
	}

	cfg := Config{
		ChunkSize:    1500,
		ChunkOverlap: 200,
		MinChunk:     50,
	}
	chunks := ChunkTree(tree, cfg)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Index != 0 {
		t.Errorf("expected index 0, got %d", chunks[0].Index)
	}
	if !strings.Contains(chunks[0].Text, "word") {
		t.Errorf("expected chunk text to contain 'word', got %q", chunks[0].Text)
	}
}

func TestChunkTree_LargeTreeRequiresSplitting(t *testing.T) {
	// Generate text that is well above ChunkSize tokens.
	// ~3000 words -> ~3990 tokens at 1.33 tokens/word.
	largeText := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 300)

	tree := &doctree.DocTree{
		Title: "Large",
		Children: []*doctree.DocNode{
			{
				Title: "Big Section",
				Text:  largeText,
			},
		},
	}

	cfg := Config{
		ChunkSize:    500,
		ChunkOverlap: 50,
		MinChunk:     10,
	}
	chunks := ChunkTree(tree, cfg)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for large text, got %d", len(chunks))
	}

	// Verify sequential indexing.
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d: expected index %d, got %d", i, i, c.Index)
		}
	}

	// Verify no chunk exceeds the target size by a large margin.
	// (Due to paragraph/sentence boundaries, slight overflows are expected.)
	for i, c := range chunks {
		tokens := EstimateTokens(c.Text)
		// Allow 2x the target as a generous ceiling.
		if tokens > cfg.ChunkSize*2 {
			t.Errorf("chunk %d: %d tokens exceeds 2x target %d", i, tokens, cfg.ChunkSize)
		}
	}
}

func TestChunkTree_BreadcrumbPropagation(t *testing.T) {
	tree := &doctree.DocTree{
		Title: "Doc",
		Children: []*doctree.DocNode{
			{
				Title: "Chapter 1",
				Children: []*doctree.DocNode{
					{
						Title: "Section 1.1",
						Text:  strings.Repeat("content ", 200),
					},
				},
			},
		},
	}

	cfg := Config{
		ChunkSize:    2000,
		ChunkOverlap: 100,
		MinChunk:     10,
	}
	chunks := ChunkTree(tree, cfg)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	bc := chunks[0].Breadcrumb
	want := []string{"Chapter 1", "Section 1.1"}
	if len(bc) != len(want) {
		t.Fatalf("expected breadcrumb %v, got %v", want, bc)
	}
	for i := range want {
		if bc[i] != want[i] {
			t.Errorf("breadcrumb[%d]: expected %q, got %q", i, want[i], bc[i])
		}
	}
}

func TestChunkTree_BreadcrumbIsolation(t *testing.T) {
	// Verify that breadcrumbs from sibling nodes don't leak into each other.
	tree := &doctree.DocTree{
		Title: "Doc",
		Children: []*doctree.DocNode{
			{
				Title: "A",
				Text:  strings.Repeat("alpha ", 200),
			},
			{
				Title: "B",
				Text:  strings.Repeat("beta ", 200),
			},
		},
	}

	cfg := Config{
		ChunkSize:    2000,
		ChunkOverlap: 100,
		MinChunk:     10,
	}
	chunks := ChunkTree(tree, cfg)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	if len(chunks[0].Breadcrumb) != 1 || chunks[0].Breadcrumb[0] != "A" {
		t.Errorf("chunk 0 breadcrumb: expected [A], got %v", chunks[0].Breadcrumb)
	}
	if len(chunks[1].Breadcrumb) != 1 || chunks[1].Breadcrumb[0] != "B" {
		t.Errorf("chunk 1 breadcrumb: expected [B], got %v", chunks[1].Breadcrumb)
	}
}

func TestChunkTree_MinChunkFiltering(t *testing.T) {
	// Text is very short — below MinChunk threshold.
	tree := &doctree.DocTree{
		Title: "Tiny",
		Children: []*doctree.DocNode{
			{
				Title: "Short",
				Text:  "Hi",
			},
		},
	}

	cfg := Config{
		ChunkSize:    1500,
		ChunkOverlap: 200,
		MinChunk:     100,
	}
	chunks := ChunkTree(tree, cfg)

	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks (below MinChunk), got %d", len(chunks))
	}
}

func TestChunkTree_EmptyTree(t *testing.T) {
	tree := &doctree.DocTree{Title: "Empty"}
	chunks := ChunkTree(tree, DefaultConfig())
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestChunkTree_DefaultConfigFallback(t *testing.T) {
	// Zero-value config should be replaced with defaults.
	tree := &doctree.DocTree{
		Title: "Doc",
		Children: []*doctree.DocNode{
			{Text: strings.Repeat("word ", 200)},
		},
	}
	chunks := ChunkTree(tree, Config{})
	// Should not panic and should produce at least one chunk.
	if len(chunks) < 1 {
		t.Errorf("expected at least 1 chunk with zero config (defaults applied), got %d", len(chunks))
	}
}

func TestChunkTree_NodeWithNoText(t *testing.T) {
	// Container node with no text, only children.
	tree := &doctree.DocTree{
		Title: "Doc",
		Children: []*doctree.DocNode{
			{
				Title: "Container",
				// No Text, just children.
				Children: []*doctree.DocNode{
					{
						Title: "Leaf",
						Text:  strings.Repeat("leaf content ", 100),
					},
				},
			},
		},
	}

	cfg := Config{
		ChunkSize:    2000,
		ChunkOverlap: 100,
		MinChunk:     10,
	}
	chunks := ChunkTree(tree, cfg)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	want := []string{"Container", "Leaf"}
	bc := chunks[0].Breadcrumb
	if len(bc) != len(want) {
		t.Fatalf("expected breadcrumb %v, got %v", want, bc)
	}
	for i := range want {
		if bc[i] != want[i] {
			t.Errorf("breadcrumb[%d]: expected %q, got %q", i, want[i], bc[i])
		}
	}
}
