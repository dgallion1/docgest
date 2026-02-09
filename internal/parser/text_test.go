package parser

import (
	"strings"
	"testing"
)

func TestTextParser_BasicParagraphSplitting(t *testing.T) {
	input := "First paragraph line one.\nFirst paragraph line two.\n\nSecond paragraph.\n\nThird paragraph."
	p := &TextParser{}
	tree, err := p.Parse(strings.NewReader(input), "notes.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tree.Title != "notes" {
		t.Errorf("expected title %q, got %q", "notes", tree.Title)
	}
	if len(tree.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(tree.Children))
	}

	want := []string{
		"First paragraph line one.\nFirst paragraph line two.",
		"Second paragraph.",
		"Third paragraph.",
	}
	for i, w := range want {
		if tree.Children[i].Text != w {
			t.Errorf("child[%d]: expected %q, got %q", i, w, tree.Children[i].Text)
		}
	}
}

func TestTextParser_EmptyInput(t *testing.T) {
	p := &TextParser{}
	tree, err := p.Parse(strings.NewReader(""), "empty.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tree.Title != "empty" {
		t.Errorf("expected title %q, got %q", "empty", tree.Title)
	}
	if len(tree.Children) != 0 {
		t.Errorf("expected 0 children for empty input, got %d", len(tree.Children))
	}
}

func TestTextParser_SingleLine(t *testing.T) {
	p := &TextParser{}
	tree, err := p.Parse(strings.NewReader("Hello world"), "single.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tree.Title != "single" {
		t.Errorf("expected title %q, got %q", "single", tree.Title)
	}
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree.Children))
	}
	if tree.Children[0].Text != "Hello world" {
		t.Errorf("expected %q, got %q", "Hello world", tree.Children[0].Text)
	}
}

func TestTextParser_MultipleBlankLines(t *testing.T) {
	// Multiple consecutive blank lines should not produce empty paragraphs.
	input := "Para one.\n\n\n\nPara two."
	p := &TextParser{}
	tree, err := p.Parse(strings.NewReader(input), "gaps.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(tree.Children))
	}
}

func TestTextParser_WhitespaceOnlyLines(t *testing.T) {
	// Lines with only whitespace should be treated as blank.
	input := "Para one.\n   \nPara two."
	p := &TextParser{}
	tree, err := p.Parse(strings.NewReader(input), "ws.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(tree.Children))
	}
}
