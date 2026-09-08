package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Node holds a parsed YAML value: nil, bool, int64, float64, string,
// map[string]Node, or []Node.
type Node = interface{}

type line struct {
	indent int
	text   string
	num    int
}

// Parse reads a restricted subset of YAML: block mappings, block sequences
// of scalars or mappings, and plain/single/double-quoted scalars. It does
// not support flow style, anchors, tags, or multi-line scalars.
func Parse(src string) (Node, error) {
	tokens, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, nil
	}
	node, pos, err := parseNode(tokens, 0, tokens[0].indent)
	if err != nil {
		return nil, err
	}
	if pos != len(tokens) {
		return nil, fmt.Errorf("line %d: unexpected indentation", tokens[pos].num)
	}
	return node, nil
}

func tokenize(src string) ([]line, error) {
	raw := strings.Split(src, "\n")
	var out []line
	for i, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		trimmed := strings.TrimLeft(l, " ")
		leadWS := l[:len(l)-len(trimmed)]
		if strings.Contains(leadWS, "\t") {
			return nil, fmt.Errorf("line %d: tabs are not allowed in indentation", i+1)
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		content := strings.TrimRight(stripComment(trimmed), " ")
		if content == "" {
			continue
		}
		out = append(out, line{indent: len(leadWS), text: content, num: i + 1})
	}
	return out, nil
}

// stripComment removes a trailing " # ..." comment, ignoring '#' that
// appears inside a quoted scalar.
func stripComment(s string) string {
	var inSingle, inDouble bool
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && inDouble && i+1 < len(s) {
			i++
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if c == '#' && !inSingle && !inDouble {
			if i == 0 || s[i-1] == ' ' {
				return s[:i]
			}
		}
	}
	return s
}

func isSeqItem(s string) bool {
	return s == "-" || strings.HasPrefix(s, "- ")
}

func parseNode(tokens []line, pos int, indent int) (Node, int, error) {
	if pos >= len(tokens) {
		return nil, pos, nil
	}
	if isSeqItem(tokens[pos].text) {
		return parseSequence(tokens, pos, indent)
	}
	return parseMapping(tokens, pos, indent)
}

func parseSequence(tokens []line, pos int, indent int) (Node, int, error) {
	items := []Node{}
	for pos < len(tokens) && tokens[pos].indent == indent && isSeqItem(tokens[pos].text) {
		rest := strings.TrimSpace(strings.TrimPrefix(tokens[pos].text, "-"))
		if rest == "" {
			if pos+1 < len(tokens) && tokens[pos+1].indent > indent {
				val, newPos, err := parseNode(tokens, pos+1, tokens[pos+1].indent)
				if err != nil {
					return nil, pos, err
				}
				items = append(items, val)
				pos = newPos
				continue
			}
			items = append(items, nil)
			pos++
			continue
		}
		if _, _, ok, _ := splitKeyValue(rest); ok {
			// "- key: value" opens a mapping whose first key lives on the
			// item line, indented to wherever it sits after "- ". Splice a
			// synthetic line in its place so parseMapping can walk the rest
			// of the item's keys, which are indented to match.
			itemIndent := indent + (len(tokens[pos].text) - len(rest))
			synthetic := line{indent: itemIndent, text: rest, num: tokens[pos].num}
			combined := append([]line{synthetic}, tokens[pos+1:]...)
			val, newPos, err := parseMapping(combined, 0, itemIndent)
			if err != nil {
				return nil, pos, err
			}
			items = append(items, val)
			pos += newPos
			continue
		}
		val, err := parseScalar(rest)
		if err != nil {
			return nil, pos, fmt.Errorf("line %d: %v", tokens[pos].num, err)
		}
		items = append(items, val)
		pos++
	}
	return items, pos, nil
}

func parseMapping(tokens []line, pos int, indent int) (Node, int, error) {
	m := map[string]Node{}
	for pos < len(tokens) && tokens[pos].indent == indent && !isSeqItem(tokens[pos].text) {
		key, valText, hasValue, err := splitKeyValue(tokens[pos].text)
		if err != nil {
			return nil, pos, fmt.Errorf("line %d: %v", tokens[pos].num, err)
		}
		lineNum := tokens[pos].num
		pos++
		if !hasValue || valText == "" {
			if pos < len(tokens) && tokens[pos].indent > indent {
				val, newPos, err := parseNode(tokens, pos, tokens[pos].indent)
				if err != nil {
					return nil, pos, err
				}
				m[key] = val
				pos = newPos
				continue
			}
			m[key] = nil
			continue
		}
		val, err := parseScalar(valText)
		if err != nil {
			return nil, pos, fmt.Errorf("line %d: %v", lineNum, err)
		}
		m[key] = val
	}
	return m, pos, nil
}

// splitKeyValue splits "key: value" on the first unquoted colon. hasValue
// is false for "key:" with nothing after it (including end of line).
func splitKeyValue(s string) (key string, val string, hasValue bool, err error) {
	var inSingle, inDouble bool
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && inDouble && i+1 < len(s) {
			i++
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if c == ':' && !inSingle && !inDouble {
			if i+1 == len(s) {
				return strings.TrimSpace(s[:i]), "", false, nil
			}
			if s[i+1] == ' ' {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true, nil
			}
		}
	}
	return "", "", false, fmt.Errorf("expected \"key: value\", got %q", s)
}

func parseScalar(s string) (Node, error) {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return unquoteDouble(s), nil
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return unquoteSingle(s), nil
	}
	switch s {
	case "~", "null", "Null", "NULL":
		return nil, nil
	case "true", "True", "TRUE":
		return true, nil
	case "false", "False", "FALSE":
		return false, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	return s, nil
}

func unquoteDouble(s string) string {
	inner := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == '\\' && i+1 < len(inner) {
			i++
			switch inner[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(inner[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func unquoteSingle(s string) string {
	inner := s[1 : len(s)-1]
	return strings.ReplaceAll(inner, "''", "'")
}

// Segment is one step of a lookup path: a mapping key, optionally followed
// by a [N] sequence index.
type Segment struct {
	Key      string
	HasIndex bool
	Index    int
}

// ParsePath splits a dot-separated path into segments. A literal dot in a
// key is written as "\.". A segment may end in "[N]" to index a sequence.
func ParsePath(path string) ([]Segment, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	var raws []string
	var cur strings.Builder
	escaped := false
	for _, r := range path {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '.' {
			raws = append(raws, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	raws = append(raws, cur.String())

	segs := make([]Segment, 0, len(raws))
	for _, raw := range raws {
		if raw == "" {
			return nil, fmt.Errorf("empty path segment in %q", path)
		}
		key := raw
		seg := Segment{}
		if j := strings.IndexByte(raw, '['); j >= 0 {
			if !strings.HasSuffix(raw, "]") {
				return nil, fmt.Errorf("malformed index in segment %q", raw)
			}
			key = raw[:j]
			if key == "" {
				return nil, fmt.Errorf("missing key before index in segment %q", raw)
			}
			n, err := strconv.Atoi(raw[j+1 : len(raw)-1])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid index in segment %q", raw)
			}
			seg.HasIndex = true
			seg.Index = n
		}
		seg.Key = key
		segs = append(segs, seg)
	}
	return segs, nil
}

// NotFoundError means the path doesn't resolve against the document, as
// opposed to the path being malformed or hitting a type mismatch. Callers
// that want to fall back to a default value on a missing path, but still
// treat other errors as fatal, can check for this with errors.As.
type NotFoundError struct {
	msg string
}

func (e *NotFoundError) Error() string { return e.msg }

// Lookup walks root following segs and returns the value found there.
func Lookup(root Node, segs []Segment) (Node, error) {
	cur := root
	for _, seg := range segs {
		m, ok := cur.(map[string]Node)
		if !ok {
			return nil, fmt.Errorf("cannot look up key %q: value is not a mapping", seg.Key)
		}
		v, found := m[seg.Key]
		if !found {
			return nil, &NotFoundError{msg: fmt.Sprintf("key %q not found", seg.Key)}
		}
		cur = v
		if seg.HasIndex {
			list, ok := cur.([]Node)
			if !ok {
				return nil, fmt.Errorf("cannot index %q: value is not a list", seg.Key)
			}
			if seg.Index >= len(list) {
				return nil, &NotFoundError{msg: fmt.Sprintf("index %d out of range for %q (len %d)", seg.Index, seg.Key, len(list))}
			}
			cur = list[seg.Index]
		}
	}
	return cur, nil
}
