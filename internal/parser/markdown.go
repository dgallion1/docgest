package parser

import (
	"bytes"
	"io"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// MarkdownParser handles Markdown files using goldmark.
type MarkdownParser struct{}

func (p *MarkdownParser) Parse(r io.Reader, filename string) (*doctree.DocTree, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	md := goldmark.New()
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)

	tree := &doctree.DocTree{
		Title: strings.TrimSuffix(strings.TrimSuffix(filename, ".md"), ".markdown"),
	}

	// Walk the AST and build a tree based on heading levels.
	// We use a stack to track the current nesting.
	type stackEntry struct {
		node  *doctree.DocNode
		level int
	}

	// Root is level 0 — all h1+ nest under it.
	root := &doctree.DocNode{Title: tree.Title}
	stack := []stackEntry{{node: root, level: 0}}

	var currentText bytes.Buffer

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

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Heading:
			flushText()
			level := node.Level
			title := string(node.Text(src))

			newNode := &doctree.DocNode{Title: title}

			// Pop stack until we find a parent with lower level.
			for len(stack) > 1 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}

			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, newNode)
			stack = append(stack, stackEntry{node: newNode, level: level})

		default:
			// Collect text content from non-heading blocks.
			t := extractText(n, src)
			if t != "" {
				if currentText.Len() > 0 {
					currentText.WriteString("\n\n")
				}
				currentText.WriteString(t)
			}
		}
	}
	flushText()

	tree.Children = root.Children
	// If there were no headings, put all text in a single child.
	if len(tree.Children) == 0 && root.Text != "" {
		tree.Children = []*doctree.DocNode{{Text: root.Text}}
	}

	return tree, nil
}

// extractText gets the text content of a goldmark AST node.
func extractText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	if n.Type() == ast.TypeBlock {
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			buf.Write(line.Value(src))
		}
	}
	// Also handle inline children.
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf.Write(t.Value(src))
			if t.HardLineBreak() || t.SoftLineBreak() {
				buf.WriteByte('\n')
			}
		} else {
			// Recurse for nested inlines.
			buf.WriteString(extractText(c, src))
		}
	}
	return strings.TrimSpace(buf.String())
}
