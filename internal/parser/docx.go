package parser

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
	"github.com/fumiama/go-docx"
)

// DOCXParser handles .docx files.
type DOCXParser struct{}

func (p *DOCXParser) Parse(r io.Reader, filename string) (*doctree.DocTree, error) {
	// go-docx needs a ReadSeeker+size, so write to temp file.
	tmp, err := os.CreateTemp("", "docgest-docx-*.docx")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	size, err := io.Copy(tmp, r)
	if err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("seek temp file: %w", err)
	}

	doc, err := docx.Parse(tmp, int64(size))
	tmp.Close()
	if err != nil {
		return nil, fmt.Errorf("parse docx: %w", err)
	}

	tree := &doctree.DocTree{
		Title: strings.TrimSuffix(filename, ".docx"),
	}

	type stackEntry struct {
		node  *doctree.DocNode
		level int
	}
	root := &doctree.DocNode{Title: tree.Title}
	stack := []stackEntry{{node: root, level: 0}}
	var currentText strings.Builder

	flushText := func() {
		t := strings.TrimSpace(currentText.String())
		if t != "" {
			top := stack[len(stack)-1].node
			if top.Text != "" {
				top.Text += "\n\n" + t
			} else {
				top.Text = t
			}
		}
		currentText.Reset()
	}

	for _, item := range doc.Document.Body.Items {
		para, ok := item.(*docx.Paragraph)
		if !ok {
			continue
		}

		// Check if paragraph has a heading style.
		level := docxHeadingLevel(para)
		text := docxParagraphText(para)

		if level > 0 && text != "" {
			flushText()
			newNode := &doctree.DocNode{Title: text}
			for len(stack) > 1 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, newNode)
			stack = append(stack, stackEntry{node: newNode, level: level})
		} else if text != "" {
			if currentText.Len() > 0 {
				currentText.WriteString("\n\n")
			}
			currentText.WriteString(text)
		}
	}
	flushText()

	tree.Children = root.Children
	if len(tree.Children) == 0 && root.Text != "" {
		tree.Children = []*doctree.DocNode{{Text: root.Text}}
	}

	return tree, nil
}

func docxHeadingLevel(para *docx.Paragraph) int {
	if para.Properties == nil || para.Properties.Style == nil {
		return 0
	}
	style := para.Properties.Style.Val
	switch {
	case strings.EqualFold(style, "Heading1") || strings.EqualFold(style, "heading 1"):
		return 1
	case strings.EqualFold(style, "Heading2") || strings.EqualFold(style, "heading 2"):
		return 2
	case strings.EqualFold(style, "Heading3") || strings.EqualFold(style, "heading 3"):
		return 3
	case strings.EqualFold(style, "Heading4") || strings.EqualFold(style, "heading 4"):
		return 4
	case strings.EqualFold(style, "Heading5") || strings.EqualFold(style, "heading 5"):
		return 5
	case strings.EqualFold(style, "Heading6") || strings.EqualFold(style, "heading 6"):
		return 6
	}
	return 0
}

func docxParagraphText(para *docx.Paragraph) string {
	var buf strings.Builder
	for _, child := range para.Children {
		run, ok := child.(*docx.Run)
		if !ok {
			continue
		}
		for _, rc := range run.Children {
			if t, ok := rc.(*docx.Text); ok {
				buf.WriteString(t.Text)
			}
		}
	}
	return strings.TrimSpace(buf.String())
}
