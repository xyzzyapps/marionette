// @tag-block architecture
// @tag layer: "validation"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["config", "parser", "models"]
// @tag description: "Linter — orphan tag detection, syntax validation, type safety checks"
//
// Package lint provides validation and linting for @tag annotations.
//
// It implements:
//   - Orphan tag detection: finds @tag-block comments whose attached code entity
//     has been removed or commented out
//   - Type safety validation: checks @tag properties against .marionetterc.json rules
//   - Syntax validation: ensures @tag syntax is well-formed
package lint

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/xyzzyapps/marionette/src/pkg/config"
	"github.com/xyzzyapps/marionette/src/pkg/output"
)

// Linter validates @tag annotations in source files.
type Linter struct {
	cfg *config.Config
}

// New creates a new Linter with the given project configuration.
func New(cfg *config.Config) *Linter {
	return &Linter{cfg: cfg}
}

// LintFile checks a single file for @tag validation issues.
func (l *Linter) LintFile(filePath string) ([]output.LintResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var results []output.LintResult
	lines := strings.Split(string(content), "\n")

	results = append(results, l.checkOrphanTags(lines, filePath)...)
	results = append(results, l.checkTagSyntax(lines, filePath)...)
	results = append(results, l.checkTagTypes(lines, filePath)...)

	return results, nil
}

// checkOrphanTags detects @tag-block comments whose subsequent code entity is missing.
func (l *Linter) checkOrphanTags(lines []string, filePath string) []output.LintResult {
	var results []output.LintResult

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "@tag-block") || strings.Contains(trimmed, "@tag-inline") {
			// Check that the next non-comment, non-empty line exists and is not itself a comment
			foundEntity := false
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "" {
					continue
				}
				// If the next line is also a comment, we've found an orphan
				if isCommentLine(nextLine) {
					continue
				}
				// Check if it looks like valid code (has at least one word character)
				if hasCodeContent(nextLine) {
					foundEntity = true
				}
				break
			}
			if !foundEntity && i < len(lines)-1 {
				results = append(results, output.LintResult{
					Severity: "warning",
					File:     filePath,
					Line:     i + 1,
					Message:  "Orphan tag: no code entity found after @tag annotation",
				})
			}
		}
	}

	return results
}

// checkTagSyntax validates @tag syntax is well-formed.
func (l *Linter) checkTagSyntax(lines []string, filePath string) []output.LintResult {
	var results []output.LintResult
	tagRe := regexp.MustCompile(`@tag-(block|inline)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "@tag-") {
			continue
		}

		// Check for malformed tags
		if tagRe.MatchString(trimmed) {
			// Should have a kind for block tags
			if strings.Contains(trimmed, "@tag-block") &&
				!strings.Contains(trimmed, "@tag-block:") &&
				!strings.Contains(trimmed, "@tag-block ") {
				results = append(results, output.LintResult{
					Severity: "info",
					File:     filePath,
					Line:     i + 1,
					Message:  "@tag-block should specify a kind (e.g., @tag-block: api-endpoint)",
				})
			}

			// Check for unclosed quotes
			if (strings.Count(trimmed, "\"")%2 != 0 ||
				strings.Count(trimmed, "`")%2 != 0) &&
				!strings.HasSuffix(trimmed, "*/") &&
				!strings.HasSuffix(trimmed, "```") {
				results = append(results, output.LintResult{
					Severity: "error",
					File:     filePath,
					Line:     i + 1,
					Message:  "Unclosed quote in @tag property value",
				})
			}
		}
	}

	return results
}

// checkTagTypes validates @tag properties against .marionetterc.json type rules.
func (l *Linter) checkTagTypes(lines []string, filePath string) []output.LintResult {
	var results []output.LintResult

	if l.cfg == nil || len(l.cfg.Rules) == 0 {
		return results
	}

	propRe := regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9_-]*)\s*:\s*(` +
		`"(?:[^"\\]|\\.)*"` + `|` +
		"`(?:[^`\\\\]|\\\\.)*`" + `|` +
		`\[(?:[^\[\]]*)\]` + `|` +
		`true|false|\d+(?:\.\d+)?` +
		`)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "@tag-") {
			continue
		}

		props := propRe.FindAllStringSubmatch(trimmed, -1)
		for _, m := range props {
			if len(m) < 3 {
				continue
			}
			key := strings.TrimSpace(m[1])
			rawVal := strings.TrimSpace(m[2])

			for _, rule := range l.cfg.Rules {
				if rule.Key != key {
					continue
				}

				// Check allowed values
				if len(rule.AllowedValues) > 0 {
					val := strings.Trim(rawVal, "\"` ")
					found := false
					for _, allowed := range rule.AllowedValues {
						if val == allowed {
							found = true
							break
						}
					}
					if !found {
						results = append(results, output.LintResult{
							Severity: "error",
							File:     filePath,
							Line:     i + 1,
							Message:  fmt.Sprintf("Invalid value %q for key %q: allowed values are %v", val, key, rule.AllowedValues),
						})
					}
				}

				// Check type
				if rule.Type != "" {
					typeErr := checkType(rawVal, rule.Type)
					if typeErr != "" {
						results = append(results, output.LintResult{
							Severity: "error",
							File:     filePath,
							Line:     i + 1,
							Message:  fmt.Sprintf("Type mismatch for key %q: %s", key, typeErr),
						})
					}
				}

				// Check for required but missing (handled in a separate pass)
				// This is checked per-property
				if rule.Required && rawVal == "" {
					results = append(results, output.LintResult{
						Severity: "error",
						File:     filePath,
						Line:     i + 1,
						Message:  fmt.Sprintf("Required property %q is missing or empty", key),
					})
				}
			}
		}
	}

	return results
}

// isCommentLine checks if a line is a code comment.
func isCommentLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*")
}

// hasCodeContent checks if a line has meaningful code content.
func hasCodeContent(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == "}" || trimmed == "{" {
		return false
	}
	return true
}

// checkType validates that a value matches the expected type.
func checkType(rawVal, expectedType string) string {
	val := strings.TrimSpace(rawVal)
	switch expectedType {
	case "string":
		if !(strings.HasPrefix(val, "\"") || strings.HasPrefix(val, "`")) {
			return "expected string, got " + val
		}
	case "number":
		re := regexp.MustCompile(`^\d+(?:\.\d+)?$`)
		if !re.MatchString(val) {
			return "expected number, got " + val
		}
	case "boolean":
		if val != "true" && val != "false" {
			return "expected boolean, got " + val
		}
	case "array":
		if !strings.HasPrefix(val, "[") {
			return "expected array, got " + val
		}
	}
	return ""
}
