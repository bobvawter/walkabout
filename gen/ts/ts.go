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
	"fmt"
	"go/types"
	"strings"

	"github.com/pkg/errors"
)

// Traversable represents a node within a data graph that we want to
// generate traversal code for.
type Traversable interface {
	// Declaration returns the source-code declaration for the node, which
	// may be nil if the underlying type of the traversable isn't named.
	Declaration() types.Object

	// withDeclaration returns a copy of the Traversable
	withDeclaration(types.Object) Traversable
}

// A Elementary Traversable has an element that must be processed in
// order to be meaningful.
type Elementary interface {
	Traversable
	Elem() Traversable
}

var (
	_ Elementary  = &Field{}
	_ Traversable = &Interface{}
	_ Traversable = &lazy{}
	_ Traversable = &Opaque{}
	_ Elementary  = &Pointer{}
	_ Elementary  = &Slice{}
	_ Traversable = &Struct{}
	_ Traversable = &Union{}
)

// A Field represents a named field within a Struct.
type Field struct {
	decl *types.Var
	elem Traversable
}

// Declaration implements Traversable.
func (f *Field) Declaration() types.Object { return f.decl }

// Elem returns the element traversable of the field.
func (f *Field) Elem() Traversable { return resolve(f.elem) }

// withDeclaration is a no-op for fields.
func (f *Field) withDeclaration(types.Object) Traversable { return f }

// An Interface represents a (possibly-named) interface.
type Interface struct {
	decl types.Object
}

// Declaration implements Traversable.
func (i *Interface) Declaration() types.Object { return i.decl }

// withDeclaration implements Traversable.
func (i *Interface) withDeclaration(decl types.Object) Traversable {
	ret := *i
	ret.decl = decl
	return &ret
}

type lazy struct {
	decl     types.Object
	resolved Traversable
	oracle   *Oracle
}

func resolve(t Traversable) Traversable {
	if lazy, ok := t.(*lazy); ok {
		return lazy.resolve()
	}
	return t
}

func (l *lazy) Declaration() types.Object { return l.decl }

func (l *lazy) withDeclaration(decl types.Object) Traversable {
	return l.resolve().withDeclaration(decl)
}

func (l *lazy) resolve() Traversable {
	if l.resolved != nil {
		return l.resolved
	}
	found, ok := l.oracle.Get(l.decl.Type())
	if !ok {
		panic(errors.Errorf("could not resolve %s to Traversable", l.decl.Id()))
	} else if found == l {
		panic(errors.Errorf("resolve caught in loop for %s", l.decl.Id()))
	}
	l.resolved = found
	l.oracle = nil
	return found
}

// A Pointer to another traversable type.
type Pointer struct {
	decl types.Object
	elem Traversable
}

// Declaration implements Traversable.
func (p *Pointer) Declaration() types.Object { return p.decl }

// Elem returns the element type of the pointer
func (p *Pointer) Elem() Traversable { return resolve(p.elem) }

// withDeclaration implements Traversable.
func (p *Pointer) withDeclaration(decl types.Object) Traversable {
	ret := *p
	ret.decl = decl
	return &ret
}

// A Slice of another traversable type.
type Slice struct {
	decl types.Object
	elem Traversable
}

// Declaration implements Traversable.
func (s *Slice) Declaration() types.Object { return s.decl }

// Elem returns the element type of the slice.
func (s *Slice) Elem() Traversable { return resolve(s.elem) }

// withDeclaration implements Traversable.
func (s *Slice) withDeclaration(decl types.Object) Traversable {
	ret := *s
	ret.decl = decl
	return &ret
}

// A Struct represents a (possibly-named) struct type.
type Struct struct {
	decl   types.Object
	fields []*Field
}

// Declaration implements Traversable.
func (s *Struct) Declaration() types.Object { return s.decl }

// Fields returns the fields defined by the struct.
func (s *Struct) Fields(into []*Field) []*Field { return append(into, s.fields...) }

// withDeclaration implements Traversable.
func (s *Struct) withDeclaration(decl types.Object) Traversable {
	ret := *s
	ret.decl = decl
	return &ret
}

// An Opaque type is used to represent non-traversable underlying types.
type Opaque struct {
	decl types.Object
}

// Declaration implements Traversable.
func (o *Opaque) Declaration() types.Object { return o.decl }

// withDeclaration implements Traversable.
func (o *Opaque) withDeclaration(decl types.Object) Traversable {
	ret := *o
	ret.decl = decl
	return &ret
}

// A Union represents a synthetic union interface.
type Union struct {
	name string
}

// Declaration always returns nil because the Union interface is synthetic.
func (u *Union) Declaration() types.Object { return nil }

// withDeclaration is a no-op for Union.
func (u *Union) withDeclaration(types.Object) Traversable { return u }

// QualifiedName returns a source representation of the traversable type.
func QualifiedName(pkg string, target Traversable) string {
	sb := strings.Builder{}

	for {
		if field, ok := target.(*Field); ok {
			return field.decl.Name()
		} else if decl := target.Declaration(); decl != nil {
			if pkg != "" {
				sb.WriteString(pkg)
				sb.WriteString(".")
			}
			sb.WriteString(decl.Name())
			return sb.String()
		} else {
			switch t := target.(type) {
			case *Pointer:
				sb.WriteString("*")
				target = t.Elem()
			case *Slice:
				sb.WriteString("[]")
				target = t.Elem()
			case *Union:
				if pkg != "" {
					sb.WriteString(pkg)
					sb.WriteString(".")
				}
				sb.WriteString(t.name)
				return sb.String()
			default:
				sb.WriteString(fmt.Sprintf("< anonymous %T >", t))
				return sb.String()
			}
		}
	}
}

func (f *Field) String() string     { return QualifiedName("", f) }
func (i *Interface) String() string { return QualifiedName("", i) }
func (o *Opaque) String() string    { return QualifiedName("", o) }
func (p *Pointer) String() string   { return QualifiedName("", p) }
func (s *Slice) String() string     { return QualifiedName("", s) }
func (s *Struct) String() string    { return QualifiedName("", s) }
func (u *Union) String() string     { return QualifiedName("", u) }
