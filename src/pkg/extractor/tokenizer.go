// @tag-block architecture
// @tag layer: "transform"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["models"]
// @tag description: "EBNF grammar @tag tokenizer — typed values, hierarchical keys, multiline strings, @tag-use template expansion"
package extractor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ParsedTag struct {
	TagType    string
	Kind       string
	Properties map[string]interface{}
}

type Tokenizer struct {
	tagBlockRe  *regexp.Regexp
	tagInlineRe *regexp.Regexp
	propRe      *regexp.Regexp
	numberRe    *regexp.Regexp
}

func buildPropRe() *regexp.Regexp {
	key := `([a-zA-Z][a-zA-Z0-9_/\-]*)`

	// Triple-double-quoted multiline: """..."""
	tqDq := `"{3}(?:[^"]|""?[^"])*"{3}`
	// Triple-backtick multiline: ```...```
	tqBt := "`{3}(?:[^`]|``?[^`])*`{3}"
	// Regular double-quoted: "..."
	dq := `"(?:[^"\\]|\\.)*"`
	// Regular backtick: `...`
	bt := "`(?:[^`\\\\]|\\\\.)*`"
	// Array: [...]
	arr := `\[(?:[^\[\]]*)\]`
	// Scalar: true, false, number
	scalar := `true|false|\d+(?:\.\d+)?`
	// Raw: anything without spaces, quotes, brackets, or backtick
	raw := `[^\s",\[\]` + "`" + `]+`

	val := `(` + tqDq + `|` + tqBt + `|` + dq + `|` + bt + `|` + arr + `|` + scalar + `|` + raw + `)`
	full := key + `\s*:\s*` + val
	return regexp.MustCompile(full)
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		tagBlockRe:  regexp.MustCompile(`@tag-block(?::\s*([a-zA-Z][a-zA-Z0-9_/-]*)|[:\s]+([a-zA-Z][a-zA-Z0-9_/-]*))?`),
		tagInlineRe: regexp.MustCompile(`@tag-inline`),
		propRe:      buildPropRe(),
		numberRe:    regexp.MustCompile(`^\d+(?:\.\d+)?$`),
	}
}

func (t *Tokenizer) Tokenize(strippedComment string) (*ParsedTag, error) {
	tag := &ParsedTag{Properties: make(map[string]interface{})}
	text := strings.TrimSpace(strippedComment)

	blockMatch := t.tagBlockRe.FindStringSubmatch(text)
	inlineMatch := t.tagInlineRe.FindString(text)

	if blockMatch != nil {
		tag.TagType = "block"
		if len(blockMatch) > 1 && blockMatch[1] != "" {
			tag.Kind = blockMatch[1]
		} else if len(blockMatch) > 2 && blockMatch[2] != "" {
			tag.Kind = blockMatch[2]
		}
	} else if inlineMatch != "" {
		tag.TagType = "inline"
	} else {
		return nil, fmt.Errorf("no @tag-block or @tag-inline found in comment")
	}

	propMatches := t.propRe.FindAllStringSubmatch(text, -1)
	for _, m := range propMatches {
		if len(m) < 3 {
			continue
		}
		key := strings.TrimSpace(m[1])
		rawValue := strings.TrimSpace(m[2])
		value, err := t.parseValue(rawValue)
		if err != nil {
			tag.Properties[key] = rawValue
			continue
		}
		tag.Properties[key] = value
	}
	return tag, nil
}

func (t *Tokenizer) parseValue(raw string) (interface{}, error) {
	raw = strings.TrimSpace(raw)

	// Triple-double-quoted: """..."""
	if strings.HasPrefix(raw, `"""`) && strings.HasSuffix(raw, `"""`) && len(raw) >= 6 {
		return raw[3 : len(raw)-3], nil
	}

	// Triple-backtick: ```...```
	if len(raw) >= 6 && raw[:3] == "```" && raw[len(raw)-3:] == "```" {
		return raw[3 : len(raw)-3], nil
	}

	// Single backtick: `...`
	if len(raw) >= 2 && raw[0] == '`' && raw[len(raw)-1] == '`' {
		return raw[1 : len(raw)-1], nil
	}

	// Double-quoted: "..."
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		s, err := strconv.Unquote(raw)
		if err != nil {
			return raw[1 : len(raw)-1], nil
		}
		return s, nil
	}

	// Arrays
	if len(raw) >= 2 && raw[0] == '[' && raw[len(raw)-1] == ']' {
		return t.parseArray(raw)
	}

	if raw == "true" {
		return true, nil
	}
	if raw == "false" {
		return false, nil
	}
	if t.numberRe.MatchString(raw) {
		if strings.Contains(raw, ".") {
			f, err := strconv.ParseFloat(raw, 64)
			if err == nil {
				return f, nil
			}
		} else {
			i, err := strconv.ParseInt(raw, 10, 64)
			if err == nil {
				return i, nil
			}
		}
	}
	return raw, nil
}

func (t *Tokenizer) parseArray(raw string) ([]interface{}, error) {
	inner := raw[1 : len(raw)-1]
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return []interface{}{}, nil
	}
	var items []interface{}
	current := strings.Builder{}
	inString := false
	stringChar := byte(0)
	escape := false
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		if escape {
			current.WriteByte(ch)
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if inString {
			current.WriteByte(ch)
			if ch == stringChar {
				inString = false
			}
			continue
		}
		switch ch {
		case '"', '`':
			inString = true
			stringChar = ch
			current.WriteByte(ch)
		case ',':
			item := strings.TrimSpace(current.String())
			if item != "" {
				val, err := t.parseValue(item)
				if err == nil {
					items = append(items, val)
				} else {
					items = append(items, item)
				}
			}
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	item := strings.TrimSpace(current.String())
	if item != "" {
		val, err := t.parseValue(item)
		if err == nil {
			items = append(items, val)
		} else {
			items = append(items, item)
		}
	}
	return items, nil
}

func (t *Tokenizer) ParseAll(strippedComment string) ([]*ParsedTag, error) {
	text := strings.TrimSpace(strippedComment)
	if text == "" {
		return nil, nil
	}

	// Expand @tag-use references before parsing.
	// @tag-use template-name pulls in all properties from the referenced template.
	// This is resolved at indexing time using the project's .marionetterc.json templates.
	// If the template is defined, its properties are merged into the current tag.
	text = expandTagUse(text)

	blockCount := strings.Count(text, "@tag-block")
	inlineCount := strings.Count(text, "@tag-inline")
	if blockCount+inlineCount <= 1 {
		tag, err := t.Tokenize(text)
		if err != nil {
			return nil, err
		}
		return []*ParsedTag{tag}, nil
	}

	var tags []*ParsedTag
	re := regexp.MustCompile(`(@tag-block(?:\s*:\s*[a-zA-Z][a-zA-Z0-9_/-]*)?|@tag-inline)`)
	parts := re.Split(text, -1)
	matches := re.FindAllString(text, -1)
	for i, m := range matches {
		content := ""
		if i < len(parts)-1 {
			content = parts[i+1]
		}
		tagText := strings.TrimSpace(m + content)
		if tagText == "" {
			continue
		}
		tag, err := t.Tokenize(tagText)
		if err != nil {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// Templates holds registered tag templates for @tag-use expansion.
// Set by the extractor before parsing.
var Templates = make(map[string]map[string]string)

// expandTagUse replaces @tag-use <name> with the template's properties.
// A line like:   @tag-use billing-endpoint
// is replaced with the template's key:value pairs from Templates["billing-endpoint"].
func expandTagUse(text string) string {
	re := regexp.MustCompile(`@tag-use\s+([a-zA-Z][a-zA-Z0-9_-]*)`)
	for {
		match := re.FindStringSubmatch(text)
		if match == nil {
			break
		}
		tmplName := match[1]
		props, ok := Templates[tmplName]
		if !ok || len(props) == 0 {
			// Template not found; leave @tag-use in place, tokenizer will skip it
			break
		}
		// Replace @tag-use <name> with the template's properties
		var expansion string
		for k, v := range props {
			expansion += fmt.Sprintf("\n@tag %s: \"%s\"", k, v)
		}
		text = strings.Replace(text, match[0], expansion, 1)
	}
	return text
}
