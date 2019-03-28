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

package ts

import (
	"go/types"
	"sort"
)

// Oracle holds typesystem information.
type Oracle struct {
	// We store nil values if a type isn't handled.
	data map[types.Type]Traversable
	pkg  *types.Package

	// See discussion in Get() for why these maps exist.
	ptrsTo   map[types.Type]Traversable
	slicesOf map[types.Type]Traversable
}

// NewOracle constructs a new Oracle instance.
func NewOracle(pkg *types.Package) *Oracle {
	return &Oracle{
		data:     make(map[types.Type]Traversable),
		pkg:      pkg,
		ptrsTo:   make(map[types.Type]Traversable),
		slicesOf: make(map[types.Type]Traversable),
	}
}

// All returns all traversable types.
func (o *Oracle) All() []Traversable {
	var ret []Traversable
	for _, t := range o.data {
		if t != nil {
			ret = append(ret, t)
		}
	}
	return ret
}

// Get creates or returns a Traversable that hold extracted information
// about the given type.
func (o *Oracle) Get(typ types.Type) (_ Traversable, ok bool) {
	// Simple case.
	if found, ok := o.data[typ]; ok {
		return found, found != nil
	}
	// This is a hack to improve developer convenience. When we get
	// instances of types.Type from the type-checker, they've all been
	// canonicalized, so we can use pointer references for identity.  We
	// perform a similar canonicalization so that a caller can do the
	// obvious thing and call Oracle.Get(types.NewPointer(someType)) and
	// be guaranteed to receive the same instance each time.
	if ptr, ok := typ.(*types.Pointer); ok {
		if found, ok := o.ptrsTo[ptr.Elem()]; ok {
			return found, found != nil
		}
	}
	if sl, ok := typ.(*types.Slice); ok {
		if found, ok := o.slicesOf[sl.Elem()]; ok {
			return found, found != nil
		}
	}

	// We need to record the return value in the map before recursing
	// in order to handle cyclical types. This function will be called
	// with a nil value if it is determined that a type is unsupported.
	record := func(t Traversable) {
		o.data[typ] = t
		if ptr, ok := typ.(*types.Pointer); ok {
			o.ptrsTo[ptr.Elem()] = t
		}
		if sl, ok := typ.(*types.Slice); ok {
			o.slicesOf[sl.Elem()] = t
		}
	}

	switch t := typ.(type) {
	case *types.Named:
		// We only want to consider exported types that are defined within the
		// target package.
		decl := t.Obj()
		if !decl.Exported() || decl.Pkg() != o.pkg {
			record(nil)
			return nil, false
		}
		record(&lazy{decl: decl, oracle: o})
		if under, ok := o.Get(decl.Type().Underlying()); ok {
			ret := under.withDeclaration(decl)
			record(ret)
			return ret, true
		}
		record(nil)
		return nil, false

	case *types.Interface:
		ret := &Interface{}
		record(ret)
		return ret, true

	case *types.Pointer:
		ret := &Pointer{}
		record(ret)
		if elem, ok := o.Get(t.Elem()); ok {
			ret.elem = elem
			return ret, true
		}
		record(nil)
		return nil, false

	case *types.Slice:
		ret := &Slice{}
		record(ret)
		if elem, ok := o.Get(t.Elem()); ok {
			ret.elem = elem
			return ret, true
		}
		record(nil)
		return nil, false

	case *types.Struct:
		ret := &Struct{
			fields: make([]*Field, 0, t.NumFields()),
		}
		record(ret)

		for i, j := 0, t.NumFields(); i < j; i++ {
			f := t.Field(i)
			if !f.Exported() {
				continue
			}
			if elem, ok := o.Get(f.Type()); ok {
				ret.fields = append(ret.fields, &Field{decl: f, elem: elem})
			}
		}
		return ret, true

	default:
		ret := &Opaque{}
		record(ret)
		return ret, true
	}
}

// VisitableFrom returns the declared types which should be considered
// visitable from any of the declared seed types. Whether or not a type
// is visitable is approximately equal to asking whether or not it is
// assignable, with the caveat that this method will consider pointer
// receivers for interface types. There is also special handling for
// a synthetic union interface (represented by *Union).
//
// The returned slice will be returned with stable ordering.
func (o *Oracle) VisitableFrom(seeds ...Traversable) []Traversable {
	seedSet := make(map[types.Object]bool)
	intfs := make(map[*types.Interface]bool)
	isUnion := false

	for _, s := range seeds {
		if _, ok := s.(*Union); ok {
			isUnion = true
		}
		if decl := s.Declaration(); decl != nil {
			seedSet[decl] = true
			if intf, ok := decl.Type().Underlying().(*types.Interface); ok {
				intfs[intf] = true
			}
		}
	}

	temp := make(map[Traversable]bool)
	for _, t := range o.data {
		if t == nil {
			continue
		}
		decl := t.Declaration()
		if decl == nil {
			continue
		}
		if isUnion {
			temp[t] = true
		} else if seedSet[decl] {
			temp[t] = true
		} else {
			for intf := range intfs {
				if types.Implements(decl.Type(), intf) {
					temp[t] = true
					break
				} else if ptr := types.NewPointer(decl.Type()); types.Implements(ptr, intf) {
					found, _ := o.Get(ptr)
					temp[found] = true
					break
				}
			}
		}
	}

	ret := make([]Traversable, len(temp))
	idx := 0
	for t := range temp {
		ret[idx] = t
		idx++
	}

	sort.Slice(ret, func(i, j int) bool {
		return QualifiedName("", ret[i]) < QualifiedName("", ret[j])
	})

	return ret
}
