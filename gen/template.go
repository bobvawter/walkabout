// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.

package gen

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/cockroachdb/walkabout/gen/view"

	"github.com/cockroachdb/walkabout/gen/ts"

	"github.com/cockroachdb/walkabout/gen/templates"
	"github.com/pkg/errors"
)

// generateAPI is the main code-generation function. It evaluates
// the embedded template and then calls go/format on the resulting
// code.
func (g *generation) generateAPI(root *ts.T, view *view.View, test bool, union bool) error {
	// The go template package will iterate over maps with sortable keys
	// in a stable order.
	type TMap map[string]*ts.T

	allTypes := make(TMap)
	byKind := map[ts.Kind]TMap{
		ts.Interface: make(TMap),
		ts.Pointer:   make(TMap),
		ts.Slice:     make(TMap),
		ts.Struct:    make(TMap),
	}
	opaque := make(TMap)

	for t := range view.Traversable {
		allTypes[t.String()] = t
		if tr := t.Traversable(); tr == nil {
			opaque[t.String()] = t
		} else {
			byKind[tr.Kind()][t.String()] = t
		}
	}

	// Instantiable returns all declared, visitable, instantiable types.
	// This is the contents of Visitable, without interface types.
	instantiable := make(TMap)
	for t := range view.Visitable {
		if t.Traversable().Kind() == ts.Interface {
			continue
		}
		for t.Traversable().Kind() == ts.Pointer {
			t = t.Traversable().Elem()
		}
		instantiable[t.String()] = t
	}

	visitable := make(TMap)
	for t := range view.Visitable {
		visitable[t.String()] = t
	}

	// funcMap contains a map of functions that can be called from within
	// the templates.
	var funcMap = template.FuncMap{
		// Where are we?
		"AllTypes":     func() TMap { return allTypes },
		"Visitable":    func() TMap { return visitable },
		"Instantiable": func() TMap { return instantiable },
		"Package":      func() string { return path.Base(root.Declaration().Pkg().Name()) },
		"Root":         func() *ts.T { return root },

		// Filtered data.
		"Interfaces": func() TMap { return byKind[ts.Interface] },
		"Opaques":    func() TMap { return opaque },
		"Pointers":   func() TMap { return byKind[ts.Pointer] },
		"Slices":     func() TMap { return byKind[ts.Slice] },
		"Structs":    func() TMap { return byKind[ts.Struct] },

		// More filters.
		"ImplementorsOf": func(t *ts.T) TMap {
			ret := make(TMap)
			for _, impl := range view.Interfaces[t] {
				ret[impl.String()] = impl
			}
			return ret
		},
		"TraversableFields": func(t *ts.T) []*ts.Field {
			source := t.Traversable().Fields()
			ret := source[:0]
			for _, field := range t.Traversable().Fields() {
				if view.Traversable[field.Type()] != nil {
					ret = append(ret, field)
				}
			}
			return ret
		},

		// Simple queries.
		"IsInstantiable": func(v *ts.T) bool { return instantiable[v.String()] != nil },
		"IsInterface":    func(v *ts.T) bool { tr := v.Traversable(); return tr != nil && tr.Kind() == ts.Interface },
		"IsPointer":      func(v *ts.T) bool { tr := v.Traversable(); return tr != nil && tr.Kind() == ts.Pointer },
		"IsSlice":        func(v *ts.T) bool { tr := v.Traversable(); return tr != nil && tr.Kind() == ts.Slice },
		"IsStruct":       func(v *ts.T) bool { tr := v.Traversable(); return tr != nil && tr.Kind() == ts.Struct },
		"IsTraversable":  func(v *ts.T) bool { return view.Traversable[v] != nil },
		"IsUnion":        func(v *ts.T) bool { return union && v == root },
		"IsVisitable":    func(v *ts.T) bool { return view.Visitable[v] != nil },

		// Code-generation helpers.

		// t returns an un-exported named based on the visitable interface name.
		"t": func(name string) string {
			intfName := root.String()
			return fmt.Sprintf("%s%s%s", strings.ToLower(intfName[:1]), intfName[1:], name)
		},
		// T returns an exported named based on the visitable interface name.
		"T":      func(name string) string { return fmt.Sprintf("%s%s", root, name) },
		"TypeID": func(t *ts.T) TypeID { return typeID(root, t) },
	}

	var allTemplates = make(map[string]*template.Template)
	for name, src := range templates.TemplateSources {
		allTemplates[name] = template.Must(template.New(name).Funcs(funcMap).Parse(src))
	}

	// Parse each template and sort the keys.
	sorted := make([]string, 0, len(allTemplates))
	var err error
	for key := range allTemplates {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)

	// Execute each template in sorted order.
	var buf bytes.Buffer
	for _, key := range sorted {
		if err := allTemplates[key].ExecuteTemplate(&buf, key, nil); err != nil {
			return errors.Wrap(err, key)
		}
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		println(buf.String())
		return err
	}

	outName := g.outFile
	if outName == "" {
		outName = strings.ToLower(ts.QualifiedName("", root)) + "_walkabout.g"
		if test {
			outName += "_test"
		}
		outName += ".go"
		outName = filepath.Join(g.dir, outName)
	}

	out, err := g.writeCloser(outName)
	if err != nil {
		return err
	}

	_, err = out.Write(formatted)
	if x := out.Close(); x != nil && err == nil {
		err = x
	}
	return err
}

// TypeID is a constant string to be emitted in the generated code.
type TypeID string

func (s TypeID) String() string { return string(s) }

// typeID generates a reasonable description of a type. Generated tokens
// are attached to the underlying visitation so that we can be sure
// to actually generate them in a subsequent pass.
//   *Foo -> FooPtr
//   []Foo -> FooSlice
//   []*Foo -> FooPtrSlice
//   *[]Foo -> FooSlicePtr
func typeID(root, target *ts.T) TypeID {
	suffix := ""
	for {
		if decl := target.Declaration(); decl != nil {
			return TypeID(fmt.Sprintf("%sType%s%s", root, target, suffix))
		}
		switch target.Traversable().Kind() {
		case ts.Pointer:
			suffix = "Ptr" + suffix
			target = target.Traversable().Elem()
		case ts.Slice:
			suffix = "Slice" + suffix
			target = target.Traversable().Elem()
		default:
			return "Zero"
		}
	}
}
