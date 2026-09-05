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

It does not handle flow style (`{a: 1}`, `[1, 2]`), anchors/aliases, or
multi-line block scalars (`|`, `>`). Feeding it a file that uses those will
produce a clear parse error rather than a wrong answer.

## Building

```
go build ./...
```

No dependencies outside the standard library.
