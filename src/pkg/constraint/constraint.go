// @tag-block architecture
// @tag layer: "enforcement"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["config", "parser"]
// @tag description: "Constraint engine — tag rules (require/if-then), code rules (forbidden AST calls), arch rules (import boundaries)"
//
// Package constraint provides an extensible rule engine for enforcing
// architectural, code-quality, and tag-metadata constraints across codebases.
//
// Rules are defined in .marionetterc.json and evaluated by marionette lint --strict.
// Three rule types are supported:
//
//   tag:   Validates @tag annotation metadata (required keys, allowed values)
//   code:  AST pattern matching (e.g., "no malloc in safety-critical code")
//   arch:  Module/import boundaries (e.g., "billing must not import reporting")
package constraint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xyzzyapps/marionette/src/pkg/config"
	"github.com/xyzzyapps/marionette/src/pkg/parser"
	sitter "github.com/smacker/go-tree-sitter"
)

// RuleType categorizes a constraint rule.
type RuleType string

const (
	RuleTag  RuleType = "tag"
	RuleCode RuleType = "code"
	RuleArch RuleType = "arch"
)

// Severity level for constraint violations.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Info    Severity = "info"
)

// ConstraintRule defines a single constraint to enforce.
type ConstraintRule struct {
	Type        RuleType `json:"type"`
	Name        string   `json:"name"`
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Violation records a single constraint breach.
type Violation struct {
	File     string
	Line     int
	RuleName string
	Message  string
	Severity Severity
}

// Engine evaluates constraint rules against source files and tag metadata.
type Engine struct {
	rules []ConstraintRule
}

// New creates a constraint engine from config rules.
// Converts config.RuleConfig entries into ConstraintRules.
func New(cfg *config.Config) *Engine {
	e := &Engine{}

	if cfg == nil {
		return e
	}

	// Convert existing tag validation rules
	for _, r := range cfg.Rules {
		rule := ConstraintRule{
			Type:     RuleTag,
			Name:     fmt.Sprintf("tag:%s", r.Key),
			Rule:     fmt.Sprintf("%s:%s", r.Key, strings.Join(r.AllowedValues, ",")),
			Severity: Error,
		}
		if r.Required {
			rule.Description = fmt.Sprintf("Tag key %q is required", r.Key)
		}
		if len(r.AllowedValues) > 0 {
			rule.Description = fmt.Sprintf("Tag %q must be one of: %v", r.Key, r.AllowedValues)
		}
		e.rules = append(e.rules, rule)
	}

	// Add custom constraints from new config format
	if cfg.Constraints != nil {
		for _, rc := range cfg.Constraints {
			rt := RuleTag
			switch rc.Type {
			case "code":
				rt = RuleCode
			case "arch":
				rt = RuleArch
			}
			sev := Error
			switch rc.Severity {
			case "warning":
				sev = Warning
			case "info":
				sev = Info
			}
			e.rules = append(e.rules, ConstraintRule{
				Type:        rt,
				Name:        rc.Name,
				Rule:        rc.Rule,
				Severity:    sev,
				Description: rc.Description,
			})
		}
	}

	return e
}

// Rules returns the list of loaded constraint rules.
func (e *Engine) Rules() []ConstraintRule {
	return e.rules
}

// Evaluate runs all constraint rules against the given tags and files.
// Tags argument is a slice of tag metadata maps. Files argument is paths
// to source files for code-level analysis.
func (e *Engine) Evaluate(tags []map[string]interface{}, files []string) []Violation {
	var violations []Violation

	for _, rule := range e.rules {
		switch rule.Type {
		case RuleTag:
			violations = append(violations, e.evaluateTagRule(rule, tags)...)
		case RuleCode:
			violations = append(violations, e.evaluateCodeRule(rule, files)...)
		case RuleArch:
			violations = append(violations, e.evaluateArchRule(rule, files)...)
		}
	}

	return violations
}

// evaluateTagRule checks metadata constraints like "pii-tagged must have gdpr-impact".
func (e *Engine) evaluateTagRule(rule ConstraintRule, tags []map[string]interface{}) []Violation {
	var violations []Violation

	for _, tag := range tags {
		file := tag["_file"].(string)
		line := 0
		if l, ok := tag["_line"].(int); ok {
			line = l
		}

		switch {
		case strings.HasPrefix(rule.Rule, "require:"):
			// require:key — tag must have this key
			requiredKey := strings.TrimPrefix(rule.Rule, "require:")
			if _, ok := tag[requiredKey]; !ok {
				violations = append(violations, Violation{
					File: file, Line: line, RuleName: rule.Name,
					Message:  fmt.Sprintf("Missing required tag key %q", requiredKey),
					Severity: rule.Severity,
				})
			}

		case strings.HasPrefix(rule.Rule, "if-then:"):
			// if-then:condition_key=condition_val,required_key
			parts := strings.TrimPrefix(rule.Rule, "if-then:")
			segments := strings.SplitN(parts, ",", 2)
			if len(segments) == 2 {
				cond := strings.SplitN(segments[0], "=", 2)
				requiredKey := segments[1]
				if len(cond) == 2 {
					if val, ok := tag[cond[0]]; ok && fmt.Sprintf("%v", val) == cond[1] {
						if _, ok := tag[requiredKey]; !ok {
							violations = append(violations, Violation{
								File: file, Line: line, RuleName: rule.Name,
								Message:  fmt.Sprintf("Tag with %s=%s must also have %q", cond[0], cond[1], requiredKey),
								Severity: rule.Severity,
							})
						}
					}
				}
			}

		case strings.Contains(rule.Rule, ":"):
			// key:allowed_val1,allowed_val2 — validate allowed values
			parts := strings.SplitN(rule.Rule, ":", 2)
			if len(parts) == 2 {
				key := parts[0]
				allowed := strings.Split(parts[1], ",")
				if val, ok := tag[key]; ok {
					valStr := fmt.Sprintf("%v", val)
					found := false
					for _, a := range allowed {
						if valStr == a {
							found = true
							break
						}
					}
					if !found {
						violations = append(violations, Violation{
							File: file, Line: line, RuleName: rule.Name,
							Message:  fmt.Sprintf("Tag %q has invalid value %q (allowed: %v)", key, valStr, allowed),
							Severity: rule.Severity,
						})
					}
				} else if strings.Contains(rule.Rule, "required") {
					violations = append(violations, Violation{
						File: file, Line: line, RuleName: rule.Name,
						Message:  fmt.Sprintf("Missing required tag key %q", key),
						Severity: rule.Severity,
					})
				}
			}
		}
	}

	return violations
}

// evaluateCodeRule checks AST-level constraints like "no malloc calls".
// Uses tree-sitter to parse each file and search for forbidden patterns.
func (e *Engine) evaluateCodeRule(rule ConstraintRule, files []string) []Violation {
	var violations []Violation

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		lang, err := parser.DetectLanguage(filePath)
		if err != nil {
			continue
		}

		sitParser := sitter.NewParser()
		sitParser.SetLanguage(lang.Grammar)
		tree := sitParser.Parse(nil, content)
		if tree == nil {
			continue
		}

		root := tree.RootNode()
		ruleLower := strings.ToLower(rule.Rule)

		switch {
		case strings.Contains(ruleLower, "no malloc"):
			e.findForbiddenCalls(root, content, []string{"malloc", "calloc", "realloc", "free"}, &violations, filePath, rule)

		case strings.Contains(ruleLower, "no printf"):
			e.findForbiddenCalls(root, content, []string{"printf", "sprintf", "fprintf"}, &violations, filePath, rule)

		case strings.Contains(ruleLower, "no unsafe"):
			e.findForbiddenKeywords(root, content, "unsafe", &violations, filePath, rule)

		case strings.Contains(ruleLower, "no goto"):
			e.findForbiddenKeywords(root, content, "goto", &violations, filePath, rule)

		default:
			// Generic: treat rule as a forbidden function call pattern
			forbidden := extractForbiddenNames(rule.Rule)
			if len(forbidden) > 0 {
				e.findForbiddenCalls(root, content, forbidden, &violations, filePath, rule)
			}
		}

		tree.Close()
		sitParser.Close()
	}

	return violations
}

// findForbiddenCalls searches for function calls with forbidden names.
func (e *Engine) findForbiddenCalls(root *sitter.Node, content []byte, funcs []string, violations *[]Violation, filePath string, rule ConstraintRule) {
	forbid := make(map[string]bool)
	for _, f := range funcs {
		forbid[f] = true
	}
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		// Check call expressions
		if node.Type() == "call_expression" || node.Type() == "call" {
			if node.NamedChildCount() > 0 {
				callee := node.NamedChild(0)
				calleeName := string(content[callee.StartByte():callee.EndByte()])
				if forbid[calleeName] || forbid[strings.TrimPrefix(calleeName, "libc_")] {
					*violations = append(*violations, Violation{
						File: filePath, Line: int(node.StartPoint().Row) + 1,
						RuleName: rule.Name,
						Message:  fmt.Sprintf("Forbidden call to %q (violates: %s)", calleeName, rule.Rule),
						Severity: rule.Severity,
					})
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(root)
}

// findForbiddenKeywords searches for keywords like "unsafe", "goto".
func (e *Engine) findForbiddenKeywords(root *sitter.Node, content []byte, keyword string, violations *[]Violation, filePath string, rule ConstraintRule) {
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node.Type() == keyword {
			*violations = append(*violations, Violation{
				File: filePath, Line: int(node.StartPoint().Row) + 1,
				RuleName: rule.Name,
				Message:  fmt.Sprintf("Forbidden keyword %q (violates: %s)", keyword, rule.Rule),
				Severity: rule.Severity,
			})
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}
	walk(root)
}

// evaluateArchRule checks architectural constraints like "module A cannot import B".
func (e *Engine) evaluateArchRule(rule ConstraintRule, files []string) []Violation {
	var violations []Violation

	// Rule format: "module_a_path must not import module_b_path"
	parts := strings.Split(rule.Rule, " must not import ")
	if len(parts) != 2 {
		return violations
	}
	forbiddenImporter := parts[0]
	forbiddenImport := parts[1]

	for _, filePath := range files {
		if !strings.Contains(filePath, forbiddenImporter) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
				if strings.Contains(trimmed, forbiddenImport) {
					violations = append(violations, Violation{
						File: filePath, Line: i + 1,
						RuleName: rule.Name,
						Message:  fmt.Sprintf("%s must not import %s", filepath.Base(filePath), forbiddenImport),
						Severity: rule.Severity,
					})
				}
			}
		}
	}

	return violations
}

// extractForbiddenNames parses a rule string for forbidden function names.
func extractForbiddenNames(rule string) []string {
	rule = strings.ToLower(rule)
	var names []string
	// Pattern: no func1, func2, func3
	if strings.HasPrefix(rule, "no ") {
		rest := strings.TrimPrefix(rule, "no ")
		// Remove "in ..." suffix
		if idx := strings.LastIndex(rest, " in "); idx > 0 {
			rest = rest[:idx]
		}
		for _, n := range strings.Split(rest, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// FormatViolations formats constraint violations as a human-readable string.
func FormatViolations(violations []Violation) string {
	if len(violations) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Constraint Violations:\n")
	for _, v := range violations {
		b.WriteString(fmt.Sprintf("  [%s] %s:%d — %s (%s)\n",
			v.Severity, v.File, v.Line, v.Message, v.RuleName))
	}
	return b.String()
}
