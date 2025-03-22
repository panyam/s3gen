package s3gen

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type TOCNode struct {
	ID       string    `json:"id"`
	Level    int       `json:"level"`
	Text     string    `json:"text"`
	Children []TOCNode `json:"children,omitempty"`
}

type TOCTransformer struct {
	TOC        []TOCNode
	CurrentIDs map[string]int
}

func NewTOCTransformer() *TOCTransformer {
	return &TOCTransformer{
		TOC:        []TOCNode{},
		CurrentIDs: make(map[string]int),
	}
}

// Transform traverses the AST and collects heading elements
func (t *TOCTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// Check if it's a heading node
		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		// Extract heading text for the title
		var text bytes.Buffer
		for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				text.Write(textNode.Segment.Value(reader.Source()))
			}
		}
		headingText := text.String()

		currid, found := heading.AttributeString("id")
		id := ""
		if !found {
		} else if sid, ok := currid.([]byte); ok && sid != nil && len(sid) > 0 {
			id = strings.TrimSpace(string(sid))
		}

		if id == "" {
			// Generate an ID based on the heading text
			id = generateID(headingText, t.CurrentIDs)
		}

		t.CurrentIDs[id]++

		// Set ID attribute for the heading node
		heading.SetAttribute([]byte("id"), []byte(id))

		// Create TOC node
		tocNode := TOCNode{
			ID:    id,
			Level: heading.Level,
			Text:  headingText,
		}

		// Add to appropriate level in the TOC
		t.addToTOC(tocNode)

		return ast.WalkContinue, nil
	})
}

// addToTOC adds a heading node to the TOC tree at the appropriate level
func (t *TOCTransformer) addToTOC(node TOCNode) {
	// For the first node or level 1 headings, add directly to the root
	if len(t.TOC) == 0 || node.Level == 1 {
		t.TOC = append(t.TOC, node)
		return
	}

	// Otherwise, find where this node belongs in the tree
	currentLevel := 0
	for i := len(t.TOC) - 1; i >= 0; i-- {
		if addToChildren(&t.TOC[i], node, currentLevel+1) {
			return
		}
	}

	// If we couldn't add it as a child of existing nodes, add it to the root
	t.TOC = append(t.TOC, node)
}

// addToChildren recursively tries to add a node to the appropriate place in the tree
func addToChildren(parent *TOCNode, node TOCNode, currentLevel int) bool {
	// If this node is at a level directly below the parent, add it as a child
	if node.Level == parent.Level+1 {
		parent.Children = append(parent.Children, node)
		return true
	}

	// If this is at a level deeper than the parent's direct children,
	// try to add it to the children's children
	if node.Level > parent.Level+1 && len(parent.Children) > 0 {
		// Try to add to the last child first
		lastChild := len(parent.Children) - 1
		if addToChildren(&parent.Children[lastChild], node, currentLevel+1) {
			return true
		}
	}

	return false
}

// generateID creates a URL-friendly ID from heading text
func generateID(text string, existingIDs map[string]int) string {
	// Convert to lowercase and replace spaces with hyphens
	id := strings.ToLower(text)

	// Replace non-alphanumeric characters with hyphens
	var result strings.Builder
	for i, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		} else if i > 0 && result.Len() > 0 && result.String()[result.Len()-1] != '-' {
			result.WriteRune('-')
		}
	}

	id = result.String()

	// Trim hyphens from beginning and end
	id = strings.Trim(id, "-")

	// Ensure no empty IDs
	if id == "" {
		id = "heading"
	}

	// Handle duplicates by adding a number
	baseID := id
	count, exists := existingIDs[baseID]
	if exists {
		id = fmt.Sprintf("%s-%d", baseID, count+1)
	}

	return id
}
