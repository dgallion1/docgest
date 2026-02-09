package doctree

// DocTree is the root of a parsed document.
type DocTree struct {
	Title    string     // Document title (from metadata or filename)
	Children []*DocNode // Top-level sections
}

// DocNode is a recursive section in the document tree.
type DocNode struct {
	Title    string     // Section heading (empty for leaf text)
	Text     string     // Text content of this node (may be empty for container nodes)
	Page     int        // Source page/line (0 if N/A)
	Children []*DocNode // Subsections
}

// Chunk is a sized text segment with structural context, ready for extraction.
type Chunk struct {
	Text       string   // Chunk text content
	Index      int      // Sequence number within document
	Breadcrumb []string // Heading hierarchy, e.g. ["Financial Results", "Revenue", "Q4"]
	PageStart  int
	PageEnd    int
}
