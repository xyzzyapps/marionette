// @tag-block architecture
// @tag layer: "input"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["parser"]
// @tag description: "Comment stripping, adjacency detection, entity name extraction, NextNamedSibling resolution"
package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// isAdjacentComment checks if two comment nodes are adjacent.
func isAdjacentComment(curr, next *sitter.Node) bool {
	return next.StartByte() > curr.EndByte() &&
		next.StartPoint().Row <= curr.EndPoint().Row+1
}

// findNextEntitySibling finds the next non-comment sibling node.
func findNextEntitySibling(node *sitter.Node) *sitter.Node {
	current := node
	for {
		next := current.NextNamedSibling()
		if next == nil {
			parent := current.Parent()
			if parent == nil {
				return nil
			}
			parentNext := parent.NextNamedSibling()
			if parentNext == nil {
				return nil
			}
			next = parentNext
		}
		switch next.Type() {
		case "comment", "line_comment", "block_comment":
			current = next
			continue
		}
		return next
	}
}

// extractEntityName extracts the name of an entity node.
func extractEntityName(node *sitter.Node, content []byte) *string {
	var nameNode *sitter.Node
	// Look for identifier/name children in common positions
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		t := child.Type()
		if t == "identifier" || t == "property_identifier" ||
			t == "type_identifier" || t == "name" {
			nameNode = child
			break
		}
	}
	if nameNode == nil && node.NamedChildCount() > 0 {
		nameNode = node.NamedChild(0)
	}
	if nameNode != nil {
		name := string(content[nameNode.StartByte():nameNode.EndByte()])
		return &name
	}
	return nil
}

// stripComment removes language-specific comment markers.
func stripComment(raw string, lang LanguageInfo) string {
	lines := strings.Split(raw, "\n")
	var stripped []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Handle all comment syntaxes
		if strings.HasPrefix(trimmed, "///") {
			trimmed = strings.TrimPrefix(trimmed, "///")
		} else if strings.HasPrefix(trimmed, "//") {
			trimmed = strings.TrimPrefix(trimmed, "//")
		} else if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimPrefix(trimmed, "#")
		} else if strings.HasPrefix(trimmed, "--") {
			trimmed = strings.TrimPrefix(trimmed, "--")
		} else if strings.HasPrefix(trimmed, ";") {
			trimmed = strings.TrimPrefix(trimmed, ";")
		} else if strings.HasPrefix(trimmed, "/*") {
			trimmed = strings.TrimPrefix(trimmed, "/*")
		} else if strings.HasPrefix(trimmed, "*/") {
			continue
		} else if strings.HasPrefix(trimmed, "*") {
			trimmed = strings.TrimPrefix(trimmed, "*")
		} else if strings.HasPrefix(trimmed, "{{!") || strings.HasPrefix(trimmed, "{#") {
			// Handle handlebars/jinja comment markers
			if strings.Contains(trimmed, "#}") {
				trimmed = strings.TrimPrefix(trimmed, "{#")
				trimmed = strings.TrimSuffix(trimmed, "#}")
			} else if strings.Contains(trimmed, "!}}") {
				trimmed = strings.TrimPrefix(trimmed, "{{!")
				trimmed = strings.TrimSuffix(trimmed, "!}}")
			}
		}
		stripped = append(stripped, strings.TrimSpace(trimmed))
	}
	return strings.Join(stripped, "\n")
}
