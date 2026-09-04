package main

import (
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
			name:    "sequence of mappings is not supported yet",
			yaml:    "items:\n  - name: a\n    value: 1\n",
			path:    "items[0]",
			wantErr: true,
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
