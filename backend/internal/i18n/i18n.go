// Package i18n localizes presentation text without changing saved analytical data.
package i18n

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed messages.json
var catalogJSON []byte

type Entry struct {
	EN string `json:"en"`
	KK string `json:"kk"`
}

var catalog map[string]Entry
var placeholders = regexp.MustCompile(`%[%sdgftw]|%\.[0-9]+[fg]`)

type rule struct {
	source string
	entry  Entry
	group  int
	count  int
}

var rules []rule
var combined *regexp.Regexp

func init() {
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		panic(err)
	}
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	patterns := make([]string, 0, len(keys))
	group := 1
	for _, key := range keys {
		var pattern strings.Builder
		position, count := 0, 0
		for _, span := range placeholders.FindAllStringIndex(key, -1) {
			pattern.WriteString(regexp.QuoteMeta(key[position:span[0]]))
			token := key[span[0]:span[1]]
			switch {
			case token == "%%":
				pattern.WriteString("%")
			case token == "%d":
				pattern.WriteString(`(-?[0-9]+)`)
				count++
			case token == "%t":
				pattern.WriteString(`(true|false)`)
				count++
			case strings.HasSuffix(token, "g") || strings.HasSuffix(token, "f"):
				pattern.WriteString(`([-+]?[0-9]+(?:\.[0-9]+)?(?:[eE][-+]?[0-9]+)?)`)
				count++
			default:
				if span[1] == len(key) {
					pattern.WriteString(`([^\n]+)`)
				} else {
					pattern.WriteString(`([^\n]+?)`)
				}
				count++
			}
			position = span[1]
		}
		pattern.WriteString(regexp.QuoteMeta(key[position:]))
		patterns = append(patterns, "("+pattern.String()+")")
		rules = append(rules, rule{key, catalog[key], group, count})
		group += count + 1
	}
	combined = regexp.MustCompile(strings.Join(patterns, "|"))
}

// Parse honors weighted Accept-Language preferences and regional language tags.
func Parse(header string) string {
	best, quality := "en", -1.0
	for _, part := range strings.Split(header, ",") {
		bits := strings.Split(strings.TrimSpace(part), ";")
		tag := strings.ToLower(strings.Split(bits[0], "-")[0])
		q := 1.0
		for _, parameter := range bits[1:] {
			if strings.HasPrefix(strings.TrimSpace(parameter), "q=") {
				var err error
				q, err = strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(parameter), "q="), 64)
				if err != nil {
					q = 0
				}
			}
		}
		if (tag == "en" || tag == "ru" || tag == "kk") && q > 0 && q <= 1 && q > quality {
			best, quality = tag, q
		}
	}
	return best
}

type contextKey struct{}

func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, contextKey{}, Parse(locale))
}
func Locale(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return "en"
}
func Language(ctx context.Context) string {
	return map[string]string{"en": "English", "ru": "Russian", "kk": "Kazakh"}[Locale(ctx)]
}

func Text(text, locale string) string { return translate(text, Parse(locale), 0) }
func translate(text, locale string, depth int) string {
	if locale == "ru" || depth > 4 {
		return text
	}
	matches := combined.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	var output strings.Builder
	position := 0
	for _, match := range matches {
		output.WriteString(text[position:match[0]])
		for _, r := range rules {
			if match[2*r.group] < 0 {
				continue
			}
			template := r.entry.EN
			if locale == "kk" {
				template = r.entry.KK
			}
			index := 0
			output.WriteString(placeholders.ReplaceAllStringFunc(template, func(token string) string {
				if token == "%%" {
					return "%"
				}
				index++
				g := r.group + index
				if index > r.count {
					return token
				}
				value := text[match[2*g]:match[2*g+1]]
				if token == "%w" || token == "%s" {
					return translate(value, locale, depth+1)
				}
				return value
			}))
			break
		}
		position = match[1]
	}
	output.WriteString(text[position:])
	return output.String()
}

// RawMessage keeps every JSON number byte-for-byte: identifiers and amounts must
// never pass through float64 while localizing descriptive fields.
func JSON(data []byte, locale string) ([]byte, error) {
	if Parse(locale) == "ru" {
		return data, nil
	}
	return localize(data, locale, false)
}

var fields = map[string]bool{"message": true, "evidence": true, "why": true, "hypothesis": true, "description": true, "request": true, "reason": true, "limitations": true, "facts": true}

func localize(data []byte, locale string, textual bool) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty JSON")
	}
	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			return nil, err
		}
		for key, value := range object {
			translated, err := localize(value, locale, fields[key])
			if err != nil {
				return nil, err
			}
			object[key] = translated
		}
		return json.Marshal(object)
	case '[':
		var array []json.RawMessage
		if err := json.Unmarshal(data, &array); err != nil {
			return nil, err
		}
		for i, value := range array {
			translated, err := localize(value, locale, textual)
			if err != nil {
				return nil, err
			}
			array[i] = translated
		}
		return json.Marshal(array)
	case '"':
		if textual {
			var value string
			if err := json.Unmarshal(data, &value); err != nil {
				return nil, err
			}
			return json.Marshal(Text(value, locale))
		}
	}
	return data, nil
}
func CSV(data []byte, locale string) ([]byte, error) {
	if Parse(locale) == "ru" {
		return data, nil
	}
	reader := csv.NewReader(bytes.NewReader(data))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return data, nil
	}
	for _, row := range rows[1:] {
		for col, header := range rows[0] {
			if fields[header] {
				row[col] = Text(row[col], locale)
			}
		}
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	writer.WriteAll(rows)
	return output.Bytes(), writer.Error()
}
