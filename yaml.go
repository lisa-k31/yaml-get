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
	// block holds the already-assembled value of a "|" or ">" block
	// scalar opened on this line, or nil if this line is not one.
	block *string
}

// Parse reads a restricted subset of YAML: block mappings, block sequences
// of scalars or mappings, literal/folded block scalars, and
// plain/single/double-quoted scalars. It does not support flow style,
// anchors, or tags.
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
	for i := 0; i < len(raw); i++ {
		l := strings.TrimSuffix(raw[i], "\r")
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
		indent := len(leadWS)
		lineNum := i + 1

		if style, chomp, indentHint, ok := blockScalarIndicator(content); ok {
			text, next, err := readBlockScalar(raw, i+1, indent, style, chomp, indentHint)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", lineNum, err)
			}
			out = append(out, line{indent: indent, text: content, num: lineNum, block: &text})
			i = next - 1
			continue
		}

		out = append(out, line{indent: indent, text: content, num: lineNum})
	}
	return out, nil
}

// blockScalarIndicator reports whether content's value position (the part
// after "key: " and/or a leading "- ") holds a "|" or ">" block scalar
// header, and if so, its style, chomping indicator, and explicit
// indentation indicator (0 meaning "auto-detect").
func blockScalarIndicator(content string) (style byte, chomp byte, indentHint int, ok bool) {
	candidate := content
	if isSeqItem(candidate) {
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "-"))
	}
	if _, val, hasValue, err := splitKeyValue(candidate); err == nil {
		if !hasValue {
			return 0, 0, 0, false
		}
		candidate = val
	}
	if candidate == "" || (candidate[0] != '|' && candidate[0] != '>') {
		return 0, 0, 0, false
	}
	style = candidate[0]
	for _, c := range candidate[1:] {
		switch {
		case c == '-' || c == '+':
			if chomp != 0 {
				return 0, 0, 0, false
			}
			chomp = byte(c)
		case c >= '1' && c <= '9':
			if indentHint != 0 {
				return 0, 0, 0, false
			}
			indentHint = int(c - '0')
		default:
			return 0, 0, 0, false
		}
	}
	return style, chomp, indentHint, true
}

// readBlockScalar consumes the body of a "|" or ">" scalar starting at
// raw[start], given the indentation of the line that opened it, and
// assembles it per the usual chomping and folding rules. It returns the
// value and the index of the first raw line after the block.
func readBlockScalar(raw []string, start, headerIndent int, style, chomp byte, indentHint int) (string, int, error) {
	type contentLine struct {
		text  string
		extra bool
		blank bool
	}

	blockIndent := -1
	if indentHint > 0 {
		blockIndent = headerIndent + indentHint
	}

	var lines []contentLine
	lastNonBlank := -1
	i := start
	for ; i < len(raw); i++ {
		l := strings.TrimSuffix(raw[i], "\r")
		if strings.TrimSpace(l) == "" {
			lines = append(lines, contentLine{blank: true})
			continue
		}
		ls := len(l) - len(strings.TrimLeft(l, " "))
		if blockIndent == -1 {
			if ls <= headerIndent {
				break
			}
			blockIndent = ls
		}
		if ls < blockIndent {
			break
		}
		lines = append(lines, contentLine{text: l[blockIndent:], extra: ls > blockIndent})
		lastNonBlank = len(lines) - 1
	}

	hasContent := lastNonBlank != -1
	trailingBlanks := len(lines) - 1 - lastNonBlank
	if hasContent {
		lines = lines[:lastNonBlank+1]
	} else {
		lines = nil
	}

	// A single line break between two lines folds to a space in folded
	// style. A run of N blank lines between them is N+1 line breaks in
	// literal style (each blank line is itself a line) but just N in
	// folded style, since one of those breaks is the fold itself.
	var b strings.Builder
	started := false
	pendingBlanks := 0
	prevExtra := false
	for _, cl := range lines {
		if cl.blank {
			if started {
				pendingBlanks++
			}
			continue
		}
		if started {
			switch {
			case style == '|':
				b.WriteString(strings.Repeat("\n", pendingBlanks+1))
			case pendingBlanks > 0:
				b.WriteString(strings.Repeat("\n", pendingBlanks))
			case cl.extra || prevExtra:
				b.WriteByte('\n')
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteString(cl.text)
		started = true
		pendingBlanks = 0
		prevExtra = cl.extra
	}

	switch chomp {
	case '-':
	case '+':
		n := trailingBlanks
		if hasContent {
			n++
		}
		b.WriteString(strings.Repeat("\n", n))
	default:
		if hasContent {
			b.WriteByte('\n')
		}
	}

	return b.String(), i, nil
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
		tok := tokens[pos]
		rest := strings.TrimSpace(strings.TrimPrefix(tok.text, "-"))
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
			itemIndent := indent + (len(tok.text) - len(rest))
			synthetic := line{indent: itemIndent, text: rest, num: tok.num, block: tok.block}
			combined := append([]line{synthetic}, tokens[pos+1:]...)
			val, newPos, err := parseMapping(combined, 0, itemIndent)
			if err != nil {
				return nil, pos, err
			}
			items = append(items, val)
			pos += newPos
			continue
		}
		if tok.block != nil {
			items = append(items, *tok.block)
			pos++
			continue
		}
		val, err := parseScalar(rest)
		if err != nil {
			return nil, pos, fmt.Errorf("line %d: %v", tok.num, err)
		}
		items = append(items, val)
		pos++
	}
	return items, pos, nil
}

func parseMapping(tokens []line, pos int, indent int) (Node, int, error) {
	m := map[string]Node{}
	for pos < len(tokens) && tokens[pos].indent == indent && !isSeqItem(tokens[pos].text) {
		tok := tokens[pos]
		key, valText, hasValue, err := splitKeyValue(tok.text)
		if err != nil {
			return nil, pos, fmt.Errorf("line %d: %v", tok.num, err)
		}
		pos++
		if tok.block != nil {
			m[key] = *tok.block
			continue
		}
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
			return nil, pos, fmt.Errorf("line %d: %v", tok.num, err)
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

// Set returns src with the value at segs replaced by newValue, rewriting
// only that one line and leaving everything else - formatting, comments,
// key order, block scalars elsewhere in the file - untouched. It only
// supports mapping paths: a segment with an index, or a path that runs
// through a sequence, is reported as an error rather than guessed at,
// since rewriting inside a sequence item safely needs more of the item's
// structure than a single line carries.
//
// newValue is taken as a raw YAML scalar, the same way a value read out of
// the file would be: "true" writes a bare boolean, "" clears the key to
// null, and a value already wrapped in matching quotes is passed through
// as the caller's own quoted scalar. Anything else that would not survive
// being written bare - leading/trailing space, an embedded "#" comment, a
// stray "|"/">" at the start - is double-quoted automatically.
func Set(src string, segs []Segment, newValue string) (string, error) {
	if len(segs) == 0 {
		return "", fmt.Errorf("empty path")
	}
	tokens, err := tokenize(src)
	if err != nil {
		return "", err
	}
	if len(tokens) == 0 {
		return "", &NotFoundError{msg: fmt.Sprintf("key %q not found", segs[0].Key)}
	}
	tok, err := locateScalar(tokens, 0, tokens[0].indent, segs)
	if err != nil {
		return "", err
	}

	key, _, _, err := splitKeyValue(tok.text)
	if err != nil {
		return "", fmt.Errorf("line %d: %v", tok.num, err)
	}
	val := formatSetValue(key, newValue)

	newLine := strings.Repeat(" ", tok.indent) + key + ":"
	if val != "" {
		newLine += " " + val
	}

	rawLines := strings.Split(src, "\n")
	if tok.num-1 >= len(rawLines) {
		return "", fmt.Errorf("line %d: out of range", tok.num)
	}
	if strings.HasSuffix(rawLines[tok.num-1], "\r") {
		newLine += "\r"
	}
	rawLines[tok.num-1] = newLine
	return strings.Join(rawLines, "\n"), nil
}

// locateScalar walks the mapping structure the same way parseMapping does,
// following segs, and returns the token holding the final scalar so Set
// can rewrite its value in place. Non-matching keys have their subtree
// skipped by advancing past every token indented deeper than they are,
// without needing to parse it.
func locateScalar(tokens []line, pos, indent int, segs []Segment) (*line, error) {
	seg := segs[0]
	if seg.HasIndex {
		return nil, fmt.Errorf("--set does not support indexing into a list (%q)", seg.Key)
	}
	if pos >= len(tokens) || tokens[pos].indent != indent {
		return nil, &NotFoundError{msg: fmt.Sprintf("key %q not found", seg.Key)}
	}
	if isSeqItem(tokens[pos].text) {
		return nil, fmt.Errorf("cannot look up key %q: value is not a mapping", seg.Key)
	}
	for pos < len(tokens) && tokens[pos].indent == indent && !isSeqItem(tokens[pos].text) {
		tok := &tokens[pos]
		key, valText, hasValue, err := splitKeyValue(tok.text)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", tok.num, err)
		}
		if key != seg.Key {
			pos++
			for pos < len(tokens) && tokens[pos].indent > indent {
				pos++
			}
			continue
		}
		if len(segs) == 1 {
			if tok.block != nil {
				return nil, fmt.Errorf("line %d: --set does not support rewriting a block scalar", tok.num)
			}
			if (!hasValue || valText == "") && pos+1 < len(tokens) && tokens[pos+1].indent > indent {
				return nil, fmt.Errorf("line %d: %q is a mapping or list, not a scalar", tok.num, seg.Key)
			}
			return tok, nil
		}
		if tok.block != nil || (hasValue && valText != "") {
			return nil, fmt.Errorf("cannot look up key %q: value is not a mapping", segs[1].Key)
		}
		if pos+1 >= len(tokens) || tokens[pos+1].indent <= indent {
			return nil, &NotFoundError{msg: fmt.Sprintf("key %q not found", segs[1].Key)}
		}
		if isSeqItem(tokens[pos+1].text) {
			return nil, fmt.Errorf("--set does not support paths through a list")
		}
		return locateScalar(tokens, pos+1, tokens[pos+1].indent, segs[1:])
	}
	return nil, &NotFoundError{msg: fmt.Sprintf("key %q not found", seg.Key)}
}

// formatSetValue turns a raw --set value into the text that goes after
// "key: " in the rewritten line.
func formatSetValue(key, v string) string {
	if v == "" {
		return ""
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return v
	}
	if needsQuoting(key, v) {
		return quoteDouble(v)
	}
	return v
}

// needsQuoting reports whether v would fail to round-trip if written bare
// as the value of key: reading the resulting line back through tokenize
// and splitKeyValue would either trim it, truncate it at a "#", or
// misread it as a block scalar header.
func needsQuoting(key, v string) bool {
	if v != strings.TrimSpace(v) {
		return true
	}
	if strings.ContainsAny(v, "\n\r") {
		return true
	}
	candidate := key + ": " + v
	if stripComment(candidate) != candidate {
		return true
	}
	if _, _, _, ok := blockScalarIndicator(candidate); ok {
		return true
	}
	return false
}

func quoteDouble(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

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
