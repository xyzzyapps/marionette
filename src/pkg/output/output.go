// Package output provides formatting engines for the Marionette CLI output.
//
// It supports three formats:
//   - json: Raw JSON output matching the marionette index schema
//   - markdown: Human-readable markdown tables
//   - ctags: Legacy ctags-compatible flat text format for retro-compatibility
// @tag-block architecture
// @tag layer: "format"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["models"]
// @tag description: "Output formatters — JSON, Markdown tables, legacy ctags format, lint results"
package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/xyzzyapps/marionette/src/pkg/models"
)

// Format represents the output format.
type Format string

const (
	FormatJSON     Format = "json"
	FormatMarkdown Format = "markdown"
	FormatCtags    Format = "ctags"
)

// Formatter formats tags for output.
type Formatter struct {
	format Format
}

// NewFormatter creates a new Formatter with the given format.
func NewFormatter(f Format) *Formatter {
	return &Formatter{format: f}
}

// FormatTags formats a slice of tags.
func (f *Formatter) FormatTags(tags []models.Tag) (string, error) {
	switch f.format {
	case FormatJSON:
		return f.formatJSON(tags)
	case FormatMarkdown:
		return f.formatMarkdown(tags)
	case FormatCtags:
		return f.formatCtags(tags)
	default:
		return f.formatJSON(tags)
	}
}

// FormatStats formats index statistics.
func (f *Formatter) FormatStats(meta *models.IndexMetadata) string {
	switch f.format {
	case FormatJSON:
		b, _ := json.MarshalIndent(meta, "", "  ")
		return string(b)
	default:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Index Version:  %s\n", meta.Version))
		b.WriteString(fmt.Sprintf("Total Files:    %d\n", meta.TotalFiles))
		b.WriteString(fmt.Sprintf("Total Tags:     %d\n", meta.TotalTags))
		if !meta.CreatedAt.IsZero() {
			b.WriteString(fmt.Sprintf("Created:        %s\n", meta.CreatedAt.Format("2006-01-02 15:04:05")))
		}
		if !meta.UpdatedAt.IsZero() {
			b.WriteString(fmt.Sprintf("Last Updated:   %s\n", meta.UpdatedAt.Format("2006-01-02 15:04:05")))
		}
		return b.String()
	}
}

// formatJSON returns tags as indented JSON.
func (f *Formatter) formatJSON(tags []models.Tag) (string, error) {
	b, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// formatMarkdown returns tags as a markdown table.
func (f *Formatter) formatMarkdown(tags []models.Tag) (string, error) {
	if len(tags) == 0 {
		return "No tags found.\n", nil
	}

	var b strings.Builder
	b.WriteString("| ID | File | Tag Type | Entity | Kind | Metadata |\n")
	b.WriteString("|----|------|----------|--------|------|----------|\n")

	for _, tag := range tags {
		entityName := "-"
		if tag.EntityName != nil {
			entityName = *tag.EntityName
		}
		entityKind := "-"
		if tag.EntityKind != nil {
			entityKind = *tag.EntityKind
		}
		metaStr := formatMetadata(tag.Metadata)
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			tag.ID, tag.File, tag.TagType, entityName, entityKind, metaStr))
	}

	return b.String(), nil
}

// formatCtags returns tags in a ctags-compatible flat text format.
func (f *Formatter) formatCtags(tags []models.Tag) (string, error) {
	var b strings.Builder

	// ctags format: tagname filename line_number;"
	for _, tag := range tags {
		tagName := fmt.Sprintf("%s:%s", tag.TagType, tag.ID)
		lineNum := tag.Range.Start.Line + 1 // convert to 1-based
		metaStr := formatMetadataCompact(tag.Metadata)
		b.WriteString(fmt.Sprintf("%s\t%s\t%d;\"\t%s\n",
			tagName, tag.File, lineNum, metaStr))
	}

	return b.String(), nil
}

// formatMetadata converts a metadata map to a compact string representation.
func formatMetadata(m map[string]interface{}) string {
	if len(m) == 0 {
		return "-"
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}

// formatMetadataCompact formats metadata for ctags output.
func formatMetadataCompact(m map[string]interface{}) string {
	kind := ""
	if k, ok := m["kind"]; ok {
		kind = fmt.Sprintf("%v", k)
	}
	return fmt.Sprintf("tag:%s", kind)
}

// FormatIndexOutput formats the full index output.
func (f *Formatter) FormatIndexOutput(output *models.IndexOutput) (string, error) {
	switch f.format {
	case FormatJSON:
		b, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	case FormatMarkdown:
		var b strings.Builder
		b.WriteString("# Marionette Index\n\n")
		b.WriteString("## Metadata\n\n")
		b.WriteString(f.FormatStats(&output.Metadata))
		b.WriteString("\n\n## Tags\n\n")
		table, _ := f.formatMarkdown(output.Tags)
		b.WriteString(table)
		return b.String(), nil
	case FormatCtags:
		var b strings.Builder
		for _, tag := range output.Tags {
			tagName := fmt.Sprintf("%s:%s", tag.TagType, tag.ID)
			lineNum := tag.Range.Start.Line + 1
			metaStr := formatMetadataCompact(tag.Metadata)
			b.WriteString(fmt.Sprintf("%s\t%s\t%d;\"\t%s\n",
				tagName, tag.File, lineNum, metaStr))
		}
		return b.String(), nil
	default:
		b, _ := json.MarshalIndent(output, "", "  ")
		return string(b), nil
	}
}

// FormatErrors formats a list of errors.
func FormatErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Errors:\n")
	for _, e := range errors {
		b.WriteString(fmt.Sprintf("  - %s\n", e))
	}
	return b.String()
}

// FormatLintResults formats lint validation results.
func FormatLintResults(results []LintResult) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "Severity\tFile\tLine\tMessage")
	fmt.Fprintln(w, "--------\t----\t----\t-------")

	for _, r := range results {
		lineStr := fmt.Sprintf("%d", r.Line)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Severity, r.File, lineStr, r.Message)
	}
	w.Flush()

	return b.String()
}

// LintResult represents a single lint check result.
type LintResult struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}
