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

	"github.com/cockroachdb/walkabout/gen/ts"

	"github.com/cockroachdb/walkabout/gen/templates"
	"github.com/pkg/errors"
)

// generateAPI is the main code-generation function. It evaluates
// the embedded template and then calls go/format on the resulting
// code.
func (g *generation) generateAPI(
	o *ts.Oracle, root ts.Traversable, seeds ts.TraversableSet, test bool) error {

	visitable := o.VisitableFrom(seeds)
	flattened := ts.NewTraversableSet()
	for t := range visitable {
		flattenType(root, t, flattened)
	}

	// funcMap contains a map of functions that can be called from within
	// the templates.
	var funcMap = template.FuncMap{
		"AllTypes": func() map[string]ts.Traversable {
			ret := make(map[string]ts.Traversable)
			for t := range flattened {
				ret[t.String()] = t
			}
			return ret
		},
		// Declared returns all declared, visitable types.
		"Declared": func() map[string]ts.Traversable {
			ret := make(map[string]ts.Traversable)
			for t := range flattened {
				if t.Declaration() != nil {
					ret[t.String()] = t
				}
			}
			return ret
		},
		// Instantiable returns all declared, visitable, instantiable types.
		"Instantiable": func() map[string]ts.Traversable {
			ret := make(map[string]ts.Traversable)
			for t := range flattened {
				if t.Declaration() != nil {
					switch t.(type) {
					case *ts.Opaque, *ts.Struct:
						ret[t.String()] = t
					}
				}
			}
			return ret
		},
		// Intfs returns a sortable map of all interface types used.
		"Intfs": func() map[string]*ts.Interface {
			ret := make(map[string]*ts.Interface)
			for t := range flattened {
				if s, ok := t.(*ts.Interface); ok {
					ret[s.String()] = s
				}
			}
			return ret
		},
		// IsPointer returns true if the type is a pointer.
		"IsPointer": func(v ts.Traversable) *ts.Pointer {
			ptr, _ := v.(*ts.Pointer)
			return ptr
		},
		// IsStruct returns true if the type is a struct type.
		"IsStruct": func(v ts.Traversable) *ts.Struct {
			s, _ := v.(*ts.Struct)
			return s
		},
		// IsUnion returns true if the type is a union type.
		"IsUnion": func(v ts.Traversable) *ts.Union {
			u, _ := v.(*ts.Union)
			return u
		},
		"Opaques": func() map[string]*ts.Opaque {
			ret := make(map[string]*ts.Opaque)
			for t := range flattened {
				if s, ok := t.(*ts.Opaque); ok {
					ret[s.String()] = s
				}
			}
			return ret
		},
		// Package returns the name of the package we're working in.
		"Package": func() string { return path.Base(root.Declaration().Pkg().Name()) },
		// Pointers returns a sortable map of all pointer types used.
		"Pointers": func() map[string]*ts.Pointer {
			ret := make(map[string]*ts.Pointer)
			for t := range flattened {
				if s, ok := t.(*ts.Pointer); ok {
					ret[s.String()] = s
				}
			}
			return ret
		},
		// Root returns the base traversable type.
		"Root": func() ts.Traversable {
			return root
		},
		// Slices returns a sortable map of all slice types used.
		"Slices": func() map[string]*ts.Slice {
			ret := make(map[string]*ts.Slice)
			for t := range flattened {
				if s, ok := t.(*ts.Slice); ok {
					ret[s.String()] = s
				}
			}
			return ret
		},
		// ShouldTraverse returns true if the given type is "interesting"
		// for the purposes of traversal.
		"ShouldTraverse": func(t ts.Traversable) bool {
			return seeds.ShouldTraverse(t)
		},
		// ShouldVisit returns true if the type should be passed to a user
		// facade function.
		"ShouldVisit": func(t ts.Traversable) bool {
			return visitable.Contains(t)
		},
		// Structs returns a sortable map of all slice types used.
		"Structs": func() map[string]*ts.Struct {
			ret := make(map[string]*ts.Struct)
			for t := range flattened {
				if s, ok := t.(*ts.Struct); ok {
					ret[s.String()] = s
				}
			}
			return ret
		},
		// t returns an un-exported named based on the visitable interface name.
		"t": func(name string) string {
			intfName := root.String()
			return fmt.Sprintf("%s%s%s", strings.ToLower(intfName[:1]), intfName[1:], name)
		},
		// T returns an exported named based on the visitable interface name.
		"T": func(name string) string {
			return fmt.Sprintf("%s%s", root, name)
		},
		// TypeID returns a reasonable description of a type.
		"TypeID": func(t ts.Traversable) TypeID {
			return typeID(root, t)
		},
		// VisitableFrom exposes Oracle.VisitableFrom via a sortable map.
		"VisitableFrom": func(seed ts.Traversable) map[string]ts.Traversable {
			ret := make(map[string]ts.Traversable)
			for t := range o.VisitableFrom(ts.NewTraversableSet(seed)) {
				ret[t.String()] = t
			}
			return ret
		},
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

// flattenType extracts all reachable types into the given map if it
// does not already have an entry for the target type.
func flattenType(root, target ts.Traversable, into ts.TraversableSet) {
	if _, opaque := target.(*ts.Opaque); opaque && target.Declaration() == nil {
		return
	}
	if !into.Contains(target) {
		if _, field := target.(*ts.Field); !field {
			into.Add(target)
		}
		switch t := target.(type) {
		case ts.Elementary:
			flattenType(root, t.Elem(), into)
		case *ts.Struct:
			for _, f := range t.Fields() {
				flattenType(root, f, into)
			}
		}
	}
}

// typeID generates a reasonable description of a type. Generated tokens
// are attached to the underlying visitation so that we can be sure
// to actually generate them in a subsequent pass.
//   *Foo -> FooPtr
//   []Foo -> FooSlice
//   []*Foo -> FooPtrSlice
//   *[]Foo -> FooSlicePtr
func typeID(root, target ts.Traversable) TypeID {
	suffix := ""
	for {
		if decl := target.Declaration(); decl != nil {
			return TypeID(fmt.Sprintf("%sType%s%s", root, target, suffix))
		}
		switch t := target.(type) {
		case *ts.Pointer:
			suffix = "Ptr" + suffix
			target = t.Elem()
		case *ts.Slice:
			suffix = "Slice" + suffix
			target = t.Elem()
		default:
			return "Zero"
			//			panic(errors.Errorf("unimplemented: %T", target))
		}
	}
}
