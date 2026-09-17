package main

import (
	"errors"
	"fmt"
	"testing"
)

func lookupValue(yamlSrc, path string) (Node, error) {
	root, err := Parse(yamlSrc)
	if err != nil {
		return nil, err
	}
	segs, err := ParsePath(path)
	if err != nil {
		return nil, err
	}
	return Lookup(root, segs)
}

func TestParseAndLookup(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "plain scalar",
			yaml: "name: value\n",
			path: "name",
			want: "value",
		},
		{
			name: "quoted value with colon",
			yaml: `msg: "hello: world"` + "\n",
			path: "msg",
			want: "hello: world",
		},
		{
			name: "quoted value with hash is not a comment",
			yaml: `msg: "not a # comment"` + "\n",
			path: "msg",
			want: "not a # comment",
		},
		{
			name: "trailing comment after unquoted value",
			yaml: "port: 8080 # default port\n",
			path: "port",
			want: "8080",
		},
		{
			name: "single quoted with escaped quote",
			yaml: "name: 'O''Brien'\n",
			path: "name",
			want: "O'Brien",
		},
		{
			name: "quoted number stays a string",
			yaml: `version: "1.0"` + "\n",
			path: "version",
			want: "1.0",
		},
		{
			name: "unquoted float",
			yaml: "ratio: 0.5\n",
			path: "ratio",
			want: "0.5",
		},
		{
			name: "negative int",
			yaml: "offset: -3\n",
			path: "offset",
			want: "-3",
		},
		{
			name: "double quoted escape sequence",
			yaml: `msg: "line1\nline2"` + "\n",
			path: "msg",
			want: "line1\nline2",
		},
		{
			name: "empty value is null",
			yaml: "debug:\n",
			path: "debug",
			want: "",
		},
		{
			name: "tilde is null",
			yaml: "x: ~\n",
			path: "x",
			want: "",
		},
		{
			name: "null keyword",
			yaml: "x: null\n",
			path: "x",
			want: "",
		},
		{
			name: "boolean true",
			yaml: "enabled: true\n",
			path: "enabled",
			want: "true",
		},
		{
			name: "nested mapping",
			yaml: "server:\n  host: localhost\n  port: 8080\n",
			path: "server.port",
			want: "8080",
		},
		{
			name: "list of scalars by index",
			yaml: "colors:\n  - red\n  - green\n  - blue\n",
			path: "colors[1]",
			want: "green",
		},
		{
			name: "escaped dot in path reaches literal key",
			yaml: "a.b: 1\n",
			path: `a\.b`,
			want: "1",
		},
		{
			name: "duplicate keys, last one wins",
			yaml: "x: 1\nx: 2\n",
			path: "x",
			want: "2",
		},
		{
			name: "crlf line endings",
			yaml: "name: value\r\nport: 80\r\n",
			path: "port",
			want: "80",
		},
		{
			name: "comments and blank lines interspersed",
			yaml: "# top comment\n\nname: value\n\n# trailing\n",
			path: "name",
			want: "value",
		},
		{
			name:    "tab in indentation is rejected",
			yaml:    "server:\n\thost: localhost\n",
			path:    "server.host",
			wantErr: true,
		},
		{
			name:    "missing key",
			yaml:    "name: value\n",
			path:    "missing",
			wantErr: true,
		},
		{
			name:    "index out of range",
			yaml:    "list:\n  - a\n  - b\n",
			path:    "list[5]",
			wantErr: true,
		},
		{
			name:    "descending into a scalar",
			yaml:    "name: value\n",
			path:    "name.sub",
			wantErr: true,
		},
		{
			name: "sequence of mappings, first key",
			yaml: "items:\n  - name: a\n    value: 1\n  - name: b\n    value: 2\n",
			path: "items[0].name",
			want: "a",
		},
		{
			name: "sequence of mappings, second item second key",
			yaml: "items:\n  - name: a\n    value: 1\n  - name: b\n    value: 2\n",
			path: "items[1].value",
			want: "2",
		},
		{
			name: "sequence of mappings with nested mapping value",
			yaml: "items:\n  - name: a\n    tags:\n      env: prod\n      tier: web\n",
			path: "items[0].tags.env",
			want: "prod",
		},
		{
			name: "sequence of mappings, extra indent after dash",
			yaml: "items:\n  -   name: a\n      value: 1\n",
			path: "items[0].value",
			want: "1",
		},
		{
			name: "literal block scalar",
			yaml: "msg: |\n  line one\n  line two\n",
			path: "msg",
			want: "line one\nline two\n",
		},
		{
			name: "literal block scalar strip chomping",
			yaml: "msg: |-\n  line one\n  line two\n",
			path: "msg",
			want: "line one\nline two",
		},
		{
			name: "literal block scalar keep chomping preserves trailing blanks",
			yaml: "msg: |+\n  a\n  b\n\n\nother: c\n",
			path: "msg",
			want: "a\nb\n\n\n",
		},
		{
			name: "folded block scalar",
			yaml: "msg: >\n  this will be\n  folded into\n  one line\n",
			path: "msg",
			want: "this will be folded into one line\n",
		},
		{
			name: "folded block scalar keeps blank line as break",
			yaml: "msg: >\n  first para\n\n  second para\n",
			path: "msg",
			want: "first para\nsecond para\n",
		},
		{
			name: "folded block scalar keeps more-indented lines literal",
			yaml: "msg: >\n  normal\n    literal\n  normal again\n",
			path: "msg",
			want: "normal\n  literal\nnormal again\n",
		},
		{
			name: "block scalar with explicit indentation indicator",
			yaml: "msg: |2\n    four spaces kept\n  two spaces stripped\n",
			path: "msg",
			want: "  four spaces kept\ntwo spaces stripped\n",
		},
		{
			name: "block scalar followed by a sibling key",
			yaml: "msg: |\n  line one\n  line two\nother: value\n",
			path: "other",
			want: "value",
		},
		{
			name: "empty block scalar",
			yaml: "msg: |\nother: value\n",
			path: "msg",
			want: "",
		},
		{
			name: "block scalar as a sequence item",
			yaml: "notes:\n  - |\n    first note\n  - second note\n",
			path: "notes[0]",
			want: "first note\n",
		},
		{
			name: "block scalar as first key of a sequence-of-mappings item",
			yaml: "items:\n  - text: |\n      hello\n      world\n    id: 1\n",
			path: "items[0].text",
			want: "hello\nworld\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lookupValue(tc.yaml, tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotStr := ""
			if got != nil {
				gotStr = fmt.Sprint(got)
			}
			if gotStr != tc.want {
				t.Errorf("got %q, want %q", gotStr, tc.want)
			}
		})
	}
}

// TestNotFoundError checks that Lookup reports missing paths with
// *NotFoundError specifically, so callers can offer a default value for
// those but still surface type-mismatch errors (e.g. indexing a scalar)
// as fatal.
func TestNotFoundError(t *testing.T) {
	cases := []struct {
		name        string
		yaml        string
		path        string
		wantNotFund bool
	}{
		{"missing key", "name: value\n", "missing", true},
		{"index out of range", "list:\n  - a\n  - b\n", "list[5]", true},
		{"descending into a scalar", "name: value\n", "name.sub", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := lookupValue(tc.yaml, tc.path)
			if err == nil {
				t.Fatalf("expected error, got none")
			}
			var notFound *NotFoundError
			if got := errors.As(err, &notFound); got != tc.wantNotFund {
				t.Errorf("errors.As(err, *NotFoundError) = %v, want %v (err: %v)", got, tc.wantNotFund, err)
			}
		})
	}
}

func setValue(yamlSrc, path, newValue string) (string, error) {
	segs, err := ParsePath(path)
	if err != nil {
		return "", err
	}
	return Set(yamlSrc, segs, newValue)
}

func TestSet(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		path    string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:  "replace a plain scalar",
			yaml:  "name: old\nport: 8080\n",
			path:  "name",
			value: "new",
			want:  "name: new\nport: 8080\n",
		},
		{
			name:  "replace preserves indentation and sibling lines",
			yaml:  "server:\n  host: localhost\n  port: 8080\n",
			path:  "server.port",
			value: "9090",
			want:  "server:\n  host: localhost\n  port: 9090\n",
		},
		{
			name:  "replace preserves a comment on another line",
			yaml:  "name: old # keep me\nport: 8080\n",
			path:  "port",
			value: "9090",
			want:  "name: old # keep me\nport: 9090\n",
		},
		{
			name:  "drops the comment on the edited line itself",
			yaml:  "port: 8080 # default\n",
			path:  "port",
			value: "9090",
			want:  "port: 9090\n",
		},
		{
			name:  "bare value that needs quoting gets quoted",
			yaml:  "msg: old\n",
			path:  "msg",
			value: "has a # in it",
			want:  "msg: \"has a # in it\"\n",
		},
		{
			name:  "value with leading or trailing space gets quoted",
			yaml:  "msg: old\n",
			path:  "msg",
			value: " padded ",
			want:  "msg: \" padded \"\n",
		},
		{
			name:  "value that looks like a block header gets quoted",
			yaml:  "msg: old\n",
			path:  "msg",
			value: "|",
			want:  "msg: \"|\"\n",
		},
		{
			name:  "already-quoted value is passed through as-is",
			yaml:  "msg: old\n",
			path:  "msg",
			value: `"literal true"`,
			want:  "msg: \"literal true\"\n",
		},
		{
			name:  "bare bool and int values stay unquoted",
			yaml:  "enabled: false\ncount: 1\n",
			path:  "enabled",
			value: "true",
			want:  "enabled: true\ncount: 1\n",
		},
		{
			name:  "empty value clears the key to null",
			yaml:  "debug: yes\n",
			path:  "debug",
			value: "",
			want:  "debug:\n",
		},
		{
			name:  "set on a null value fills it in",
			yaml:  "debug:\nother: 1\n",
			path:  "debug",
			value: "true",
			want:  "debug: true\nother: 1\n",
		},
		{
			name:  "crlf line endings are preserved",
			yaml:  "name: old\r\nport: 80\r\n",
			path:  "name",
			value: "new",
			want:  "name: new\r\nport: 80\r\n",
		},
		{
			name:    "missing key is an error",
			yaml:    "name: value\n",
			path:    "missing",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "cannot set a mapping",
			yaml:    "server:\n  host: localhost\n",
			path:    "server",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "cannot descend into a scalar",
			yaml:    "name: value\n",
			path:    "name.sub",
			value:   "x",
			wantErr: true,
		},
		{
			name:  "index into a sequence of scalars",
			yaml:  "colors:\n  - red\n  - green\n",
			path:  "colors[0]",
			value: "blue",
			want:  "colors:\n  - blue\n  - green\n",
		},
		{
			name:  "index into a sequence, indentation and siblings preserved",
			yaml:  "items:\n  - name: a\n    value: 1\n  - name: b\n    value: 2\n",
			path:  "items[1].value",
			value: "9",
			want:  "items:\n  - name: a\n    value: 1\n  - name: b\n    value: 9\n",
		},
		{
			name:  "index then descend two levels into a sequence item",
			yaml:  "items:\n  - name: a\n    tags:\n      env: dev\n      tier: web\n",
			path:  "items[0].tags.env",
			value: "prod",
			want:  "items:\n  - name: a\n    tags:\n      env: prod\n      tier: web\n",
		},
		{
			name:  "sequence item value needing quotes gets quoted",
			yaml:  "colors:\n  - red\n  - green\n",
			path:  "colors[1]",
			value: "has a # in it",
			want:  "colors:\n  - red\n  - \"has a # in it\"\n",
		},
		{
			name:  "clearing a sequence item to null",
			yaml:  "colors:\n  - red\n  - green\n",
			path:  "colors[0]",
			value: "",
			want:  "colors:\n  -\n  - green\n",
		},
		{
			name:    "path through a list without an index is not supported",
			yaml:    "items:\n  - name: a\n",
			path:    "items.name",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "indexing a key that is not a list",
			yaml:    "colors: red\n",
			path:    "colors[0]",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "index out of range",
			yaml:    "colors:\n  - red\n  - green\n",
			path:    "colors[5]",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "cannot set a whole sequence item that is a mapping",
			yaml:    "items:\n  - name: a\n    value: 1\n",
			path:    "items[0]",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "cannot rewrite a block scalar sequence item",
			yaml:    "notes:\n  - |\n    first\n  - second\n",
			path:    "notes[0]",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "cannot rewrite a block scalar reached through a sequence item",
			yaml:    "items:\n  - text: |\n      hello\n    id: 1\n",
			path:    "items[0].text",
			value:   "x",
			wantErr: true,
		},
		{
			name:    "cannot rewrite a block scalar",
			yaml:    "msg: |\n  line one\n",
			path:    "msg",
			value:   "x",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setValue(tc.yaml, tc.path, tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSetRoundTrip checks that the result of Set still parses, and that
// looking the path back up returns what was just written.
func TestSetRoundTrip(t *testing.T) {
	src := "server:\n  host: localhost\n  port: 8080\n# a comment\ntags:\n  - a\n  - b\n"
	newSrc, err := setValue(src, "server.host", "example.com")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := lookupValue(newSrc, "server.host")
	if err != nil {
		t.Fatalf("lookup after set: %v", err)
	}
	if got != "example.com" {
		t.Errorf("got %q, want %q", got, "example.com")
	}
	if port, err := lookupValue(newSrc, "server.port"); err != nil || fmt.Sprint(port) != "8080" {
		t.Errorf("unrelated key server.port changed: %v, %v", port, err)
	}
}

func TestParsePathErrors(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"trailing dot", "a."},
		{"leading dot", ".a"},
		{"non-numeric index", "list[abc]"},
		{"unclosed index", "list["},
		{"index without key", "[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePath(tc.path); err == nil {
				t.Fatalf("expected error for path %q, got none", tc.path)
			}
		})
	}
}
