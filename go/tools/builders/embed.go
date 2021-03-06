// Copyright 2017 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// embed generates a .go file from the contents of a list of data files. It is
// invoked by go_embed_data as an action.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"
)

var headerTpl = template.Must(template.New("embed").Parse(`// Generated by go_embed_data for {{.Label}}. DO NOT EDIT.

package {{.Package}}

{{if .Multi}}
var {{.Var}} = map[string]{{.Type}}{
{{- range $i, $f := .Sources}}
	{{$.Key $f}}: {{$.Var}}_{{$i}},
{{- end}}
}
{{end}}

`))

func main() {
	log.SetPrefix("embed: ")
	log.SetFlags(0) // don't print timestamps
	if err := run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type configuration struct {
	Label, Package, Var string
	Multi               bool
	Sources             []string
	out, workspace      string
	flatten, strData    bool
}

func (c *configuration) Type() string {
	if c.strData {
		return "string"
	} else {
		return "[]byte"
	}
}

func (c *configuration) Key(filename string) string {
	workspacePrefix := "external/" + c.workspace + "/"
	key := filepath.FromSlash(strings.TrimPrefix(filename, workspacePrefix))
	if c.flatten {
		key = path.Base(filename)
	}
	return strconv.Quote(key)
}

func run(args []string) error {
	c, err := newConfiguration(args)
	if err != nil {
		return err
	}

	f, err := os.Create(c.out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	if err := headerTpl.Execute(w, c); err != nil {
		return err
	}

	if c.Multi {
		return embedMultipleFiles(c, w)
	}
	return embedSingleFile(c, w)
}

func newConfiguration(args []string) (*configuration, error) {
	var c configuration
	flags := flag.NewFlagSet("embed", flag.ExitOnError)
	flags.StringVar(&c.Label, "label", "", "Label of the rule being executed (required)")
	flags.StringVar(&c.Package, "package", "", "Go package name (required)")
	flags.StringVar(&c.Var, "var", "", "Variable name (required)")
	flags.BoolVar(&c.Multi, "multi", false, "Whether the variable is a map or a single value")
	flags.StringVar(&c.out, "out", "", "Go file to generate (required)")
	flags.StringVar(&c.workspace, "workspace", "", "Name of the workspace (required)")
	flags.BoolVar(&c.flatten, "flatten", false, "Whether to access files by base name")
	flags.BoolVar(&c.strData, "string", false, "Whether to store contents as strings")
	flags.Parse(args[1:])
	if c.Label == "" {
		return nil, errors.New("error: -label option not provided")
	}
	if c.Package == "" {
		return nil, errors.New("error: -package option not provided")
	}
	if c.Var == "" {
		return nil, errors.New("error: -var option not provided")
	}
	if c.out == "" {
		return nil, errors.New("error: -out option not provided")
	}
	if c.workspace == "" {
		return nil, errors.New("error: -workspace option not provided")
	}
	c.Sources = flags.Args()
	if !c.Multi && len(c.Sources) != 1 {
		return nil, fmt.Errorf("error: -multi flag not given, so want exactly one source; got %d", len(c.Sources))
	}
	return &c, nil
}

func embedSingleFile(c *configuration, w io.Writer) error {
	dataBegin, dataEnd := "\"", "\"\n"
	if !c.strData {
		dataBegin, dataEnd = "[]byte(\"", "\")\n"
	}

	if _, err := fmt.Fprintf(w, "var %s = %s", c.Var, dataBegin); err != nil {
		return err
	}
	if err := embedFileContents(w, c.Sources[0]); err != nil {
		return err
	}
	_, err := fmt.Fprint(w, dataEnd)
	return err
}

func embedMultipleFiles(c *configuration, w io.Writer) error {
	if len(c.Sources) == 0 {
		return nil
	}

	dataBegin, dataEnd := "\"", "\"\n"
	if !c.strData {
		dataBegin, dataEnd = "[]byte(\"", "\")\n"
	}

	if _, err := fmt.Fprint(w, "var (\n"); err != nil {
		return err
	}
	for i, filename := range c.Sources {
		if _, err := fmt.Fprintf(w, "\t%s_%d = %s", c.Var, i, dataBegin); err != nil {
			return err
		}
		if err := embedFileContents(w, filename); err != nil {
			return err
		}
		if _, err := fmt.Fprint(w, dataEnd); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, ")\n")
	return err
}

func embedFileContents(w io.Writer, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(&escapeWriter{w}, bufio.NewReader(f))
	return err
}

type escapeWriter struct {
	w io.Writer
}

func (w *escapeWriter) Write(data []byte) (n int, err error) {
	n = len(data)

	for err == nil && len(data) > 0 {
		// https://golang.org/ref/spec#String_literals: "Within the quotes, any
		// character may appear except newline and unescaped double quote. The
		// text between the quotes forms the value of the literal, with backslash
		// escapes interpreted as they are in rune literals […]."
		switch b := data[0]; b {
		case '\\':
			_, err = w.w.Write([]byte(`\\`))
		case '"':
			_, err = w.w.Write([]byte(`\"`))
		case '\n':
			_, err = w.w.Write([]byte(`\n`))

		case '\x00':
			// https://golang.org/ref/spec#Source_code_representation: "Implementation
			// restriction: For compatibility with other tools, a compiler may
			// disallow the NUL character (U+0000) in the source text."
			_, err = w.w.Write([]byte(`\x00`))

		default:
			// https://golang.org/ref/spec#Source_code_representation: "Implementation
			// restriction: […] A byte order mark may be disallowed anywhere else in
			// the source."
			const byteOrderMark = '\uFEFF'

			if r, size := utf8.DecodeRune(data); r != utf8.RuneError && r != byteOrderMark {
				_, err = w.w.Write(data[:size])
				data = data[size:]
				continue
			}

			_, err = fmt.Fprintf(w.w, `\x%02x`, b)
		}
		data = data[1:]
	}

	return n - len(data), err
}
