package parser

import (
	"strings"
	"testing"
)

func TestMarkdownParser_HeadingHierarchy(t *testing.T) {
	input := `# Title

Intro text.

## Section A

Section A content.

### Subsection A1

Subsection A1 content.

## Section B

Section B content.
`
	p := &MarkdownParser{}
	tree, err := p.Parse(strings.NewReader(input), "doc.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tree.Title != "doc" {
		t.Errorf("expected title %q, got %q", "doc", tree.Title)
	}

	// Top-level: one h1 ("Title")
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 top-level child (h1), got %d", len(tree.Children))
	}

	h1 := tree.Children[0]
	if h1.Title != "Title" {
		t.Errorf("expected h1 title %q, got %q", "Title", h1.Title)
	}

	// h1 should have "Intro text." as its text content
	if !strings.Contains(h1.Text, "Intro text.") {
		t.Errorf("expected h1 text to contain %q, got %q", "Intro text.", h1.Text)
	}

	// h1 has two h2 children: "Section A" and "Section B"
	if len(h1.Children) != 2 {
		t.Fatalf("expected 2 h2 children, got %d", len(h1.Children))
	}

	secA := h1.Children[0]
	if secA.Title != "Section A" {
		t.Errorf("expected %q, got %q", "Section A", secA.Title)
	}
	if !strings.Contains(secA.Text, "Section A content.") {
		t.Errorf("expected section A text to contain %q, got %q", "Section A content.", secA.Text)
	}

	// Section A has one h3 child
	if len(secA.Children) != 1 {
		t.Fatalf("expected 1 h3 child under Section A, got %d", len(secA.Children))
	}
	sub := secA.Children[0]
	if sub.Title != "Subsection A1" {
		t.Errorf("expected %q, got %q", "Subsection A1", sub.Title)
	}

	secB := h1.Children[1]
	if secB.Title != "Section B" {
		t.Errorf("expected %q, got %q", "Section B", secB.Title)
	}
}

func TestMarkdownParser_NoHeadings(t *testing.T) {
	input := `Just some plain text.

Another paragraph here.`

	p := &MarkdownParser{}
	tree, err := p.Parse(strings.NewReader(input), "plain.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No headings: all text should be collected into a single child node.
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 child for headingless markdown, got %d", len(tree.Children))
	}

	text := tree.Children[0].Text
	if !strings.Contains(text, "Just some plain text.") {
		t.Errorf("expected text to contain first paragraph, got %q", text)
	}
	if !strings.Contains(text, "Another paragraph here.") {
		t.Errorf("expected text to contain second paragraph, got %q", text)
	}
}

func TestMarkdownParser_MixedContentWithCodeBlocks(t *testing.T) {
	input := "# API Reference\n\nSome intro.\n\n## Endpoints\n\nList of endpoints:\n\n```\nGET /api/users\nPOST /api/users\n```\n\nMore text after code.\n"

	p := &MarkdownParser{}
	tree, err := p.Parse(strings.NewReader(input), "api.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have one h1 child
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 top-level child, got %d", len(tree.Children))
	}

	h1 := tree.Children[0]
	if h1.Title != "API Reference" {
		t.Errorf("expected title %q, got %q", "API Reference", h1.Title)
	}

	// h1 has one h2 child: "Endpoints"
	if len(h1.Children) != 1 {
		t.Fatalf("expected 1 h2 child, got %d", len(h1.Children))
	}

	endpoints := h1.Children[0]
	if endpoints.Title != "Endpoints" {
		t.Errorf("expected title %q, got %q", "Endpoints", endpoints.Title)
	}

	// The endpoints section should contain the code block content
	if !strings.Contains(endpoints.Text, "GET /api/users") {
		t.Errorf("expected code block content in text, got %q", endpoints.Text)
	}
	if !strings.Contains(endpoints.Text, "More text after code.") {
		t.Errorf("expected post-code text, got %q", endpoints.Text)
	}
}

func TestMarkdownParser_EmptyInput(t *testing.T) {
	p := &MarkdownParser{}
	tree, err := p.Parse(strings.NewReader(""), "empty.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tree.Children) != 0 {
		t.Errorf("expected 0 children for empty input, got %d", len(tree.Children))
	}
}

func TestMarkdownParser_TitleStripping(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"readme.md", "readme"},
		{"notes.markdown", "notes"},
		{"plain.md", "plain"},
	}
	p := &MarkdownParser{}
	for _, tt := range tests {
		tree, err := p.Parse(strings.NewReader("text"), tt.filename)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt.filename, err)
		}
		if tree.Title != tt.want {
			t.Errorf("filename=%q: expected title %q, got %q", tt.filename, tt.want, tree.Title)
		}
	}
}
