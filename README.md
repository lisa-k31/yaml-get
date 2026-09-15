# yaml-get

Pull one value out of a YAML config file and print it, so a shell script
or a CI step doesn't have to shell out to `python -c "import yaml..."` or
install `yq` just to read a port number or a version string.

```
$ cat config.yaml
server:
  host: localhost
  port: 8080
  tags:
    - primary
    - west

$ yaml-get -f config.yaml server.port
8080

$ yaml-get -f config.yaml server.tags[1]
west

$ cat config.yaml | yaml-get server.host
localhost
```

If the path doesn't resolve, or resolves to a mapping or a list instead of
a scalar, `yaml-get` exits non-zero and prints nothing useful to stdout, so
it's safe to use in `set -e` scripts:

```
$ yaml-get -f config.yaml server.missing; echo "exit: $?"
yaml-get: key "missing" not found
exit: 2
```

Pass `-d`/`--default` to get a fallback value instead, for paths that are
optional in the config:

```
$ yaml-get -f config.yaml -d 9090 server.missing
9090
```

`-d` only kicks in when the path doesn't resolve. It does not suppress
errors from a malformed file or from indexing into a scalar.

## Writing values

Pass `-set` to write a value into the file instead of printing one, e.g. to
bump a version in a CI release step:

```
$ yaml-get -f config.yaml -set 9090 server.port
$ yaml-get -f config.yaml server.port
9090
```

`-set` requires `-f`; it rewrites the file in place, touching only the one
line that holds the target value and leaving every other line - including
comments and key order - exactly as it was. It only supports paths made of
mapping keys: it can't set a value reached through a `[N]` index, and it
won't overwrite an existing block (`|`/`>`) scalar or a mapping/list.

The value is written as a raw YAML scalar, the same rules used when
reading one back: `-set true` writes a bare boolean, `-set ''` clears the
key to `null`, and a value that wouldn't survive being written bare (it
has leading/trailing space, contains an unquoted `#`, or starts with `|`
or `>`) is quoted automatically. To force a string, quote it yourself:
`-set '"true"'` writes the literal three-character string, not a boolean.

## Path syntax

Paths are dot-separated keys. Use `key[N]` to index into a list. A literal
dot inside a key is written `\.`:

```
$ cat db.yaml
prod.db: postgres://...

$ yaml-get -f db.yaml 'prod\.db'
postgres://...
```

## Exit codes

- `0` - value printed
- `1` - usage error, unreadable file, or the YAML doesn't parse
- `2` - the path doesn't exist, or resolves to a non-scalar

## What YAML it understands

This is a parser for the subset of YAML that config files actually use,
not the full spec. It handles:

- block mappings and block sequences, indented with spaces (tabs in
  indentation are a parse error), including sequences of mappings
  (`- name: a` followed by more keys at the same indent as `name`)
- plain, single-quoted, and double-quoted scalars, including `#` comments
  outside of quotes and basic `\n`/`\t`/`\"`/`\\` escapes inside double
  quotes
- `null`/`~`/empty, `true`/`false`, integers, and floats, following the
  usual YAML core schema rules for plain scalars (quoting a value keeps it
  a string, so `version: "1.0"` stays `"1.0"`, not `1.0`)
- duplicate keys, where the last one wins, same as most YAML loaders
- literal (`|`) and folded (`>`) block scalars, as a mapping value or a
  sequence item, with chomping indicators (`|-`, `|+`, `>-`, `>+`) and an
  explicit indentation indicator (`|2`, `>-2`, ...)

```
$ cat notes.yaml
message: |
  line one
  line two
summary: >
  this will be
  folded into
  one line

$ yaml-get -f notes.yaml message
line one
line two

$ yaml-get -f notes.yaml summary
this will be folded into one line
```

It does not handle flow style (`{a: 1}`, `[1, 2]`) or anchors/aliases.
Feeding it a file that uses those will produce a clear parse error rather
than a wrong answer.

## Building

```
go build ./...
```

No dependencies outside the standard library.
