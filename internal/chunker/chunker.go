package chunker

import (
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
)

// Config controls chunking behavior.
type Config struct {
	ChunkSize    int // Target chunk size in tokens.
	ChunkOverlap int // Overlap between consecutive chunks in tokens.
	MinChunk     int // Minimum chunk size to emit.
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		ChunkSize:    1500,
		ChunkOverlap: 200,
		MinChunk:     100,
	}
}

// ChunkTree walks a DocTree and produces structure-aware chunks.
func ChunkTree(tree *doctree.DocTree, cfg Config) []doctree.Chunk {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 1500
	}
	if cfg.ChunkOverlap <= 0 {
		cfg.ChunkOverlap = 200
	}
	if cfg.MinChunk <= 0 {
		cfg.MinChunk = 100
	}

	var chunks []doctree.Chunk
	index := 0

	for _, child := range tree.Children {
		index = walkNode(child, nil, cfg, &chunks, index)
	}

	return chunks
}

// walkNode recursively visits DocNodes, collecting text and splitting into chunks.
func walkNode(node *doctree.DocNode, breadcrumb []string, cfg Config, chunks *[]doctree.Chunk, index int) int {
	// Build breadcrumb for this node.
	var bc []string
	bc = append(bc, breadcrumb...)
	if node.Title != "" {
		bc = append(bc, node.Title)
	}

	// If this node has text, chunk it.
	if node.Text != "" {
		tokens := EstimateTokens(node.Text)
		if tokens <= cfg.ChunkSize {
			// Fits in one chunk.
			if tokens >= cfg.MinChunk {
				*chunks = append(*chunks, doctree.Chunk{
					Text:       node.Text,
					Index:      index,
					Breadcrumb: copyBreadcrumb(bc),
					PageStart:  node.Page,
					PageEnd:    node.Page,
				})
				index++
			}
		} else {
			// Split the text.
			parts := splitText(node.Text, cfg.ChunkSize, cfg.ChunkOverlap)
			for _, part := range parts {
				if EstimateTokens(part) >= cfg.MinChunk {
					*chunks = append(*chunks, doctree.Chunk{
						Text:       part,
						Index:      index,
						Breadcrumb: copyBreadcrumb(bc),
						PageStart:  node.Page,
						PageEnd:    node.Page,
					})
					index++
				}
			}
		}
	}

	// Recurse into children.
	for _, child := range node.Children {
		index = walkNode(child, bc, cfg, chunks, index)
	}

	return index
}

// splitText breaks text into chunks of approximately targetTokens, with overlap.
func splitText(text string, targetTokens, overlapTokens int) []string {
	// Split by paragraphs first.
	paragraphs := splitByParagraphs(text)

	var result []string
	var current strings.Builder
	currentTokens := 0

	for _, para := range paragraphs {
		paraTokens := EstimateTokens(para)

		// If a single paragraph exceeds the target, split it further.
		if paraTokens > targetTokens {
			// Flush current buffer.
			if currentTokens > 0 {
				result = append(result, current.String())
				current.Reset()
				currentTokens = 0
			}
			// Split the large paragraph by sentences.
			subParts := splitBySentences(para, targetTokens, overlapTokens)
			result = append(result, subParts...)
			continue
		}

		// Would adding this paragraph exceed the target?
		if currentTokens+paraTokens > targetTokens && currentTokens > 0 {
			result = append(result, current.String())

			// Start next chunk with overlap from end of current.
			overlap := getOverlapText(current.String(), overlapTokens)
			current.Reset()
			currentTokens = 0
			if overlap != "" {
				current.WriteString(overlap)
				currentTokens = EstimateTokens(overlap)
			}
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
		currentTokens += paraTokens
	}

	if currentTokens > 0 {
		result = append(result, current.String())
	}

	return result
}

// splitByParagraphs splits on double-newlines.
func splitByParagraphs(text string) []string {
	parts := strings.Split(text, "\n\n")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitBySentences breaks a large paragraph into sentence-based chunks.
func splitBySentences(text string, targetTokens, overlapTokens int) []string {
	sentences := splitSentences(text)

	var result []string
	var current strings.Builder
	currentTokens := 0

	for _, sent := range sentences {
		sentTokens := EstimateTokens(sent)

		if currentTokens+sentTokens > targetTokens && currentTokens > 0 {
			result = append(result, current.String())
			overlap := getOverlapText(current.String(), overlapTokens)
			current.Reset()
			currentTokens = 0
			if overlap != "" {
				current.WriteString(overlap)
				currentTokens = EstimateTokens(overlap)
			}
		}

		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(sent)
		currentTokens += sentTokens
	}

	if currentTokens > 0 {
		result = append(result, current.String())
	}

	return result
}

// splitSentences does basic sentence splitting.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, r := range text {
		current.WriteRune(r)
		if (r == '.' || r == '!' || r == '?') && i+1 < len(text) && text[i+1] == ' ' {
			sentences = append(sentences, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}
	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(current.String()))
	}

	return sentences
}

// getOverlapText extracts the last N tokens worth of text for overlap.
func getOverlapText(text string, targetTokens int) string {
	words := strings.Fields(text)
	// Approximate: 1.33 tokens per word.
	targetWords := int(float64(targetTokens) / 1.33)
	if targetWords <= 0 || len(words) <= targetWords {
		return ""
	}
	return strings.Join(words[len(words)-targetWords:], " ")
}

func copyBreadcrumb(bc []string) []string {
	if len(bc) == 0 {
		return nil
	}
	out := make([]string, len(bc))
	copy(out, bc)
	return out
}

// ChunkInput is used by the worker to pass chunk data for extraction.
type ChunkInput struct {
	Text       string
	Breadcrumb []string
}
