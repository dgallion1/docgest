package parser

import (
	"bufio"
	"io"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
)

// TextParser handles plain text files.
type TextParser struct{}

func (p *TextParser) Parse(r io.Reader, filename string) (*doctree.DocTree, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var paragraphs []string
	var current strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if current.Len() > 0 {
				paragraphs = append(paragraphs, current.String())
				current.Reset()
			}
		} else {
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
		}
	}
	if current.Len() > 0 {
		paragraphs = append(paragraphs, current.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	tree := &doctree.DocTree{
		Title: strings.TrimSuffix(filename, ".txt"),
	}

	// Each paragraph becomes a child node.
	for _, para := range paragraphs {
		tree.Children = append(tree.Children, &doctree.DocNode{
			Text: para,
		})
	}

	return tree, nil
}
