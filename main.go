// Command yaml-get extracts a single scalar value from a YAML file by a
// dot-separated path, for use in shell scripts and CI without pulling in
// a full YAML toolchain.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	file := flag.String("f", "", "path to YAML file (default: stdin)")
	var defaultVal string
	flag.StringVar(&defaultVal, "d", "", "value to print if the path is not found, instead of exiting non-zero")
	flag.StringVar(&defaultVal, "default", "", "same as -d")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: yaml-get [-f file] [-d default] <path>")
		flag.PrintDefaults()
	}
	flag.Parse()

	haveDefault := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "d" || f.Name == "default" {
			haveDefault = true
		}
	})

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}
	path := args[0]

	var src []byte
	var err error
	if *file == "" || *file == "-" {
		src, err = io.ReadAll(os.Stdin)
	} else {
		src, err = os.ReadFile(*file)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(1)
	}

	root, err := Parse(string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(1)
	}

	segs, err := ParsePath(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(1)
	}

	val, err := Lookup(root, segs)
	if err != nil {
		var notFound *NotFoundError
		if haveDefault && errors.As(err, &notFound) {
			fmt.Println(defaultVal)
			return
		}
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(2)
	}

	switch v := val.(type) {
	case map[string]Node, []Node:
		fmt.Fprintf(os.Stderr, "yaml-get: value at %q is not a scalar\n", path)
		os.Exit(2)
	case nil:
		fmt.Println()
	default:
		fmt.Println(v)
	}
}
