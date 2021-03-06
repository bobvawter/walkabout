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

import "go/types"

// visitableType represents a type that we can generate visitation logic
// around:
//	* a named struct which implements the visitable interface,
//		either by-reference or by-value
//	* a named interface which implements the visitable interface
//	* a pointer to a visitable type
//	* a slice of a visitable type
//	* a named visitable type; e.g. "type Foos []Foo"
//	* TODO: a map of visitable types?
type visitableType interface {
	// Implementation returns the underlying type that we actually
	// need to be able to traverse.
	Implementation() visitableType
	// String must return a codegen-safe representation of the type.
	String() string
	Visitation() *visitation
}

var (
	_ visitableType = namedStruct{}
	_ visitableType = namedInterfaceType{}
	_ visitableType = namedVisitableType{}
	_ visitableType = pointerType{}
	_ visitableType = namedSliceType{}
	_ visitableType = unionInterface{}
)

// namedVisitableType represents a named type definition like:
//   type Foos []Foo
//   type OptFoo *Foo
type namedVisitableType struct {
	*types.Named
	Underlying visitableType
}

// Implementation returns the underlying type's implementation.
func (t namedVisitableType) Implementation() visitableType {
	return t.Underlying.Implementation()
}

// String is codegen-safe.
func (t namedVisitableType) String() string {
	return t.Obj().Name()
}

func (t namedVisitableType) Visitation() *visitation {
	return t.Underlying.Visitation()
}

// namedInterfaceType represents either the visitable interface, or
// another interface which implemnts the visitable interface.
type namedInterfaceType struct {
	*types.Named
	*types.Interface
	Union string
	v     *visitation
}

// Implementation returns the receiver.
func (t namedInterfaceType) Implementation() visitableType {
	return t
}

// String is codegen-safe.
func (t namedInterfaceType) String() string {
	if t.Union != "" {
		return t.Union
	}
	return t.Obj().Name()
}

// Visitation implements visitableType.
func (t namedInterfaceType) Visitation() *visitation {
	return t.v
}

// pointerType is a pointer to a visitableType.
type pointerType struct {
	Elem visitableType
}

// Implementation returns the receiver.
func (t pointerType) Implementation() visitableType {
	return t
}

// String is codegen-safe.
func (t pointerType) String() string {
	return "*" + t.Elem.String()
}

// Visitation implements visitableType.
func (t pointerType) Visitation() *visitation {
	return t.Elem.Visitation()
}

// namedSliceType is a slice of a visitableType.
type namedSliceType struct {
	Elem visitableType
}

// Implementation returns the receiver.
func (t namedSliceType) Implementation() visitableType {
	return t
}

// String is codegen-safe.
func (t namedSliceType) String() string {
	return "[]" + t.Elem.String()
}

// Visitation implements visitableType.
func (t namedSliceType) Visitation() *visitation {
	return t.Elem.Visitation()
}

// namedStruct represents a user-defined, named struct.
type namedStruct struct {
	*types.Named
	*types.Struct
	v *visitation
}

// Implementation returns the receiver.
func (t namedStruct) Implementation() visitableType {
	return t
}

// String is codegen-safe.
func (t namedStruct) String() string {
	return t.Obj().Name()
}

// Fields returns the visitable fields of the struct.
func (t namedStruct) Fields() []fieldInfo {
	ret := make([]fieldInfo, 0, t.NumFields())

	for a, j := 0, t.NumFields(); a < j; a++ {
		f := t.Field(a)
		// Ignore un-exported fields.
		if !f.Exported() {
			continue
		}

		// Look up `field Something` to visitableType.
		if found, ok := t.v.visitableType(f.Type(), true); ok {
			ret = append(ret, fieldInfo{
				Name:   f.Name(),
				Parent: &t,
				Target: found,
			})
		}
	}

	return ret
}

// Visitation implements visitableType.
func (t namedStruct) Visitation() *visitation {
	return t.v
}

type unionInterface struct {
	name string
	v    *visitation
}

// Implementation returns the receiver.
func (t unionInterface) Implementation() visitableType {
	return t
}

// String is codegen-safe.
func (t unionInterface) String() string {
	return t.name
}

// Visitation implements visitableType.
func (t unionInterface) Visitation() *visitation {
	return t.v
}

// fieldInfo describes a field containing a visitable type.
type fieldInfo struct {
	Name string
	// The structInfo that contains this fieldInfo.
	Parent *namedStruct
	// The contents of the field.
	Target visitableType
}

// String is codegen-safe.
func (f fieldInfo) String() string {
	return f.Name
}
