// @tag-block architecture
// @tag layer: "input"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["models"]
// @tag description: "31-language tree-sitter parser registry — recursive comment collection, AST traversal, entity resolution"
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/cue"
	"github.com/smacker/go-tree-sitter/dockerfile"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/elm"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/groovy"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/ocaml"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/sql"
	"github.com/smacker/go-tree-sitter/svelte"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/toml"
	ts "github.com/smacker/go-tree-sitter/typescript/typescript"
	tsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/yaml"
)

// LanguageInfo describes a supported programming language.
type LanguageInfo struct {
	Name            string
	Grammar         *sitter.Language
	Extensions      []string
	CommentNodeType string
}

// registry holds all supported languages.
var registry []LanguageInfo

func init() {
	registry = []LanguageInfo{
		{"javascript", javascript.GetLanguage(), []string{".js", ".jsx", ".mjs", ".cjs"}, "comment"},
		{"typescript", ts.GetLanguage(), []string{".ts"}, "comment"},
		{"tsx", tsx.GetLanguage(), []string{".tsx"}, "comment"},
		{"python", python.GetLanguage(), []string{".py"}, "comment"},
		{"rust", rust.GetLanguage(), []string{".rs"}, "line_comment"},
		{"go", golang.GetLanguage(), []string{".go"}, "comment"},
		{"java", java.GetLanguage(), []string{".java"}, "comment"},
		{"c", c.GetLanguage(), []string{".c", ".h"}, "comment"},
		{"cpp", cpp.GetLanguage(), []string{".cc", ".cpp", ".cxx", ".hpp", ".hh"}, "comment"},
		{"csharp", csharp.GetLanguage(), []string{".cs"}, "comment"},
		{"ruby", ruby.GetLanguage(), []string{".rb"}, "comment"},
		{"php", php.GetLanguage(), []string{".php"}, "comment"},
		{"swift", swift.GetLanguage(), []string{".swift"}, "comment"},
		{"kotlin", kotlin.GetLanguage(), []string{".kt", ".kts"}, "comment"},
		{"scala", scala.GetLanguage(), []string{".scala"}, "comment"},
		{"bash", bash.GetLanguage(), []string{".sh", ".bash"}, "comment"},
		{"sql", sql.GetLanguage(), []string{".sql"}, "comment"},
		{"html", html.GetLanguage(), []string{".html", ".htm"}, "comment"},
		{"css", css.GetLanguage(), []string{".css"}, "comment"},
		{"yaml", yaml.GetLanguage(), []string{".yaml", ".yml"}, "comment"},
		{"toml", toml.GetLanguage(), []string{".toml"}, "comment"},
		{"elixir", elixir.GetLanguage(), []string{".ex", ".exs"}, "comment"},
		{"elm", elm.GetLanguage(), []string{".elm"}, "comment"},
		{"lua", lua.GetLanguage(), []string{".lua"}, "comment"},
		{"groovy", groovy.GetLanguage(), []string{".groovy", ".gvy"}, "comment"},
		{"hcl", hcl.GetLanguage(), []string{".hcl", ".tf", ".tfvars"}, "comment"},
		{"dockerfile", dockerfile.GetLanguage(), []string{"Dockerfile", ".dockerfile"}, "comment"},
		{"ocaml", ocaml.GetLanguage(), []string{".ml", ".mli"}, "comment"},
		{"protobuf", protobuf.GetLanguage(), []string{".proto"}, "comment"},
		{"svelte", svelte.GetLanguage(), []string{".svelte"}, "comment"},
		{"cue", cue.GetLanguage(), []string{".cue"}, "comment"},
	}
}

// DetectLanguage detects the programming language from file extension.
func DetectLanguage(path string) (LanguageInfo, error) {
	ext := strings.ToLower(filepath.Ext(path))
	name := strings.ToLower(filepath.Base(path))

	// Exact filename match (e.g., Dockerfile)
	for _, lang := range registry {
		for _, e := range lang.Extensions {
			if strings.EqualFold(e, ext) || strings.EqualFold(e, name) {
				return lang, nil
			}
		}
	}

	return LanguageInfo{}, fmt.Errorf("unsupported file: %s", path)
}

// Entity represents a parsed code entity.
type Entity struct {
	Name        *string
	Kind        string
	StartLine   uint32
	StartColumn uint32
	EndLine     uint32
	EndColumn   uint32
}

// CommentEntityPair links a raw comment string to its associated code entity.
type CommentEntityPair struct {
	RawComment      string
	StrippedComment string
	Entity          Entity
	TagType         string
}

// Parser handles parsing of multiple language source files.
type Parser struct {
	parsers map[string]*sitter.Parser
}

// New creates a new Parser with all registered languages.
func New() (*Parser, error) {
	p := &Parser{
		parsers: make(map[string]*sitter.Parser),
	}

	for _, lang := range registry {
		sitParser := sitter.NewParser()
		sitParser.SetLanguage(lang.Grammar)
		p.parsers[lang.Name] = sitParser
	}

	return p, nil
}

// SupportedLanguages returns the list of supported language names.
func SupportedLanguages() []string {
	var names []string
	for _, l := range registry {
		names = append(names, l.Name)
	}
	return names
}

// ParseFile parses a source file and extracts comment-entity pairs.
func (p *Parser) ParseFile(path string) ([]CommentEntityPair, error) {
	lang, err := DetectLanguage(path)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return p.ParseBytes(content, lang)
}

// ParseBytes parses source content and extracts all comment-entity pairs.
func (p *Parser) ParseBytes(content []byte, lang LanguageInfo) ([]CommentEntityPair, error) {
	sitParser, ok := p.parsers[lang.Name]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang.Name)
	}

	tree := sitParser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse content")
	}
	defer tree.Close()

	var commentNodes []*sitter.Node
	collectComments(tree.RootNode(), lang.CommentNodeType, &commentNodes)

	var pairs []CommentEntityPair
	i := 0
	for i < len(commentNodes) {
		blockStart := i
		for i+1 < len(commentNodes) {
			next := commentNodes[i+1]
			curr := commentNodes[i]
			if curr.Parent() == next.Parent() && isAdjacentComment(curr, next) {
				i++
			} else {
				break
			}
		}
		blockEnd := i

		firstComment := commentNodes[blockStart]
		lastComment := commentNodes[blockEnd]

		rawComment := ""
		for j := blockStart; j <= blockEnd; j++ {
			if rawComment != "" {
				rawComment += "\n"
			}
			cn := commentNodes[j]
			rawComment += string(content[cn.StartByte():cn.EndByte()])
		}

		var tagType string
		if strings.Contains(rawComment, "@tag-block") {
			tagType = "block"
		} else if strings.Contains(rawComment, "@tag-inline") {
			tagType = "inline"
		} else {
			i++
			continue
		}

		stripped := stripComment(rawComment, lang)
		entityNode := findNextEntitySibling(lastComment)
		var entity Entity

		if entityNode != nil {
			entityName := extractEntityName(entityNode, content)
			entity = Entity{
				Name:        entityName,
				Kind:        entityNode.Type(),
				StartLine:   entityNode.StartPoint().Row,
				StartColumn: entityNode.StartPoint().Column,
				EndLine:     entityNode.EndPoint().Row,
				EndColumn:   entityNode.EndPoint().Column,
			}
		} else {
			entity = Entity{
				Kind:        lang.CommentNodeType,
				StartLine:   firstComment.StartPoint().Row,
				StartColumn: firstComment.StartPoint().Column,
				EndLine:     lastComment.EndPoint().Row,
				EndColumn:   lastComment.EndPoint().Column,
			}
		}

		pairs = append(pairs, CommentEntityPair{
			RawComment:      rawComment,
			StrippedComment: stripped,
			Entity:          entity,
			TagType:         tagType,
		})
		i++
	}
	return pairs, nil
}

// Close releases resources held by the parser.
func (p *Parser) Close() {
	for _, sitParser := range p.parsers {
		sitParser.Close()
	}
}

// collectComments recursively collects all comment nodes from the tree.
func collectComments(node *sitter.Node, commentType string, result *[]*sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == commentType {
			*result = append(*result, child)
		}
		// Only recurse into named children to avoid traversing into
		// deeply nested expressions. Comments are typically at the
		// statement/block level, not inside expressions.
		if child.NamedChildCount() > 0 || child.ChildCount() > 2 {
			collectComments(child, commentType, result)
		}
	}
}
