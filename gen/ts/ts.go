// Copyright 2019 The Cockroach Authors.
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

// Package ts contains Walkabout's type system.
package ts

import (
	"go/types"
	"strings"

	"github.com/pkg/errors"
)

//go:generate stringer -type Kind

// Kind is a traversal strategy.
type Kind int

// Traversal strategies.
const (
	_ Kind = iota
	Interface
	Slice
	Struct
	Pointer
)

// ignoredT is a singleton value for ignored types.
var ignoredT *T

func init() {
	ignoredT = &T{}
	ignoredT.pointerTo = ignoredT
	ignoredT.sliceOf = ignoredT
}

// A Field represents a field in a struct type.
type Field struct {
	name string
	typ  *T
}

// Name returns the name of the field.
func (f *Field) Name() string { return f.name }

// Type returns the type of the field.
func (f *Field) Type() *T       { return f.typ }
func (f *Field) String() string { return f.name }

// T contains the strategies for handling some type within the typesystem.
type T struct {
	declaration types.Object
	pointerTo   *T
	sliceOf     *T
	traversable *Traversable
}

// Declaration returns the source declaration of the type, if one exists.
func (t *T) Declaration() types.Object { return t.declaration }

// IsIgnored returns true if the type is a placeholder for an ignored
// or non-existent type.
func (t *T) IsIgnored() bool {
	return t == ignoredT
}

// PointerTo returns a strategy for handing a pointer to the target type.
func (t *T) PointerTo() *T {
	if t.pointerTo == nil {
		t.pointerTo = &T{
			traversable: &Traversable{
				elem: t,
				kind: Pointer,
			},
		}
	}
	return t.pointerTo
}

// SliceOf returns a strategy for handing a slice to the target type.
func (t *T) SliceOf() *T {
	if t.sliceOf == nil {
		t.sliceOf = &T{
			traversable: &Traversable{
				elem: t,
				kind: Slice,
			},
		}
	}
	return t.sliceOf
}

func (t *T) String() string { return QualifiedName("", t) }

// Traversable returns the traversal strategy for the type, if it can
// be traversed.
func (t *T) Traversable() *Traversable { return t.traversable }

// Traversable represents a node within a data graph that we want to
// generate traversal code for.
type Traversable struct {
	elem   *T
	fields []*Field
	kind   Kind
}

// Elem returns the element types.
func (t *Traversable) Elem() *T {
	switch t.kind {
	case Pointer, Slice:
		return t.elem
	default:
		panic(errors.Errorf("cannot call Elem() on %v", t.kind))
	}
}

// Fields returns the fields declared within the struct in declaration order.
func (t *Traversable) Fields() []*Field {
	if t.kind != Struct {
		panic(errors.Errorf("cannot call Fields() on %v", t.kind))
	}
	return append(t.fields[:0:0], t.fields...)
}

// Kind returns the kind of traversable object.
func (t *Traversable) Kind() Kind { return t.kind }

// QualifiedName returns a source representation of the traversable type.
func QualifiedName(pkg string, t *T) string {
	sb := strings.Builder{}

	for {
		if t.IsIgnored() {
			sb.WriteString("<IGNORED>")
			return sb.String()
		}
		if decl := t.Declaration(); decl != nil {
			if pkg != "" {
				sb.WriteString(pkg)
				sb.WriteString(".")
			}
			sb.WriteString(decl.Name())
			return sb.String()
		}

		if v := t.Traversable(); v != nil {
			switch v.Kind() {
			case Pointer:
				sb.WriteString("*")
				t = v.Elem()
			case Slice:
				sb.WriteString("[]")
				t = v.Elem()

			default:
				sb.WriteString(v.Kind().String())
				return sb.String()
			}
		} else {
			sb.WriteString("__OPAQUE__")
			return sb.String()
		}
	}
}
