// @tag-block architecture
// @tag layer: "transform"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["parser"]
// @tag description: "AST-based call graph extraction — finds function definitions, call sites, resolves caller/callee relationships"
package callgraph

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/xyzzyapps/marionette/src/pkg/parser"
)

// Call represents a single call edge.
type Call struct {
	CallerFile   string
	CallerSymbol string
	CalleeFile   string
	CalleeSymbol string
	CallLine     uint32
}

// Definition represents a symbol definition.
type Definition struct {
	File     string
	Name     string
	Kind     string
	Line     uint32
	Language string
}

// ExtractDefinitions finds all function/method/class definitions in the AST.
func ExtractDefinitions(root *sitter.Node, content []byte, lang parser.LanguageInfo) []Definition {
	var defs []Definition
	visitNode(root, content, lang.Name, &defs)
	return defs
}

// ExtractCalls finds all function/method calls in the AST.
func ExtractCalls(root *sitter.Node, content []byte, lang parser.LanguageInfo) []Call {
	var calls []Call
	visitCalls(root, content, lang.Name, "", 0, &calls)
	return calls
}

func visitNode(node *sitter.Node, content []byte, lang string, defs *[]Definition) {
	defTypes := map[string]bool{
		"function_definition":   true,
		"function_declaration":  true,
		"function_item":        true,
		"method_definition":    true,
		"class_definition":     true,
		"class_declaration":    true,
		"struct_item":          true,
		"trait_item":           true,
		"impl_item":            true,
		"enum_item":            true,
		// Go
		"func_declaration":     true,
		"method_declaration":   true,
		// Rust
		"function_signature":   true,
		// C/C++
		"declaration":          true,
	}

	if defTypes[node.Type()] {
		name := extractName(node, content)
		if name != "" {
			*defs = append(*defs, Definition{
				Name: name,
				Kind: node.Type(),
				Line: node.StartPoint().Row,
			})
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		visitNode(node.NamedChild(i), content, lang, defs)
	}
}

func visitCalls(node *sitter.Node, content []byte, lang, callerFile string, callerLine uint32, calls *[]Call) {
	callTypes := map[string]bool{
		"call_expression":     true,
		"call":               true,
		"method_invocation":   true,
		"function_call":       true,
		"invoke_expression":   true,
	}

	if callTypes[node.Type()] {
		calleeName := extractCalleeName(node, content)
		if calleeName != "" && callerFile != "" {
			*calls = append(*calls, Call{
				CallerFile:   callerFile,
				CallerSymbol: extractCallerSymbol(node, content),
				CalleeFile:   callerFile, // same-file resolution
				CalleeSymbol: calleeName,
				CallLine:     node.StartPoint().Row,
			})
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		visitCalls(node.NamedChild(i), content, lang, callerFile, callerLine, calls)
	}
}

func extractName(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		t := child.Type()
		if t == "identifier" || t == "name" || t == "property_identifier" {
			return string(content[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

func extractCalleeName(node *sitter.Node, content []byte) string {
	// For call_expression, the first child is often the function being called
	if node.NamedChildCount() > 0 {
		fnNode := node.NamedChild(0)
		// Handle chained calls like obj.method()
		if fnNode.Type() == "member_expression" || fnNode.Type() == "attribute" {
			// Extract the property/method name
			last := fnNode.NamedChild(int(fnNode.NamedChildCount()) - 1)
			if last != nil {
				return string(content[last.StartByte():last.EndByte()])
			}
		}
		return string(content[fnNode.StartByte():fnNode.EndByte()])
	}
	return ""
}

func extractCallerSymbol(node *sitter.Node, content []byte) string {
	// Walk up to find the enclosing function definition
	current := node.Parent()
	for current != nil {
		t := current.Type()
		if t == "function_definition" || t == "function_declaration" ||
			t == "function_item" || t == "method_definition" ||
			t == "func_declaration" || t == "method_declaration" {
			name := extractName(current, content)
			if name != "" {
				return name
			}
		}
		current = current.Parent()
	}
	return "global"
}

// ResolveCallerTags finds all @tag annotations on the caller function.
// Returns metadata key-value pairs as a string.
func ResolveCallerTags(callerSymbol string, tags []map[string]interface{}) string {
	var parts []string
	for _, t := range tags {
		if name, ok := t["entity_name"]; ok {
			if fmt.Sprintf("%v", name) == callerSymbol {
				for k, v := range t {
					if k != "entity_name" && k != "entity_kind" {
						parts = append(parts, fmt.Sprintf("%s=%v", k, v))
					}
				}
			}
		}
	}
	return strings.Join(parts, ", ")
}
