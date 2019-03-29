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
	"go/token"
	"go/types"
)

// Oracle holds typesystem information.
type Oracle struct {
	// We store nil values if a type isn't handled.
	data map[types.Type]Traversable
	// We limit Traversable creation to types defined within these scopes.
	scopes map[*types.Scope]bool
	union  *Union

	// See discussion in Get() for why these maps exist.
	ptrsTo   map[types.Type]Traversable
	slicesOf map[types.Type]Traversable
}

// NewOracle constructs a new Oracle instance from the types defined
// within the given package.
func NewOracle(scopes []*types.Scope) *Oracle {
	scopeMap := make(map[*types.Scope]bool, len(scopes))
	o := &Oracle{
		data:     make(map[types.Type]Traversable),
		scopes:   scopeMap,
		ptrsTo:   make(map[types.Type]Traversable),
		slicesOf: make(map[types.Type]Traversable),
	}
	for _, scope := range scopes {
		scopeMap[scope] = true
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if decl, ok := obj.(*types.TypeName); ok {
				o.Get(decl.Type())
			}
		}
	}
	return o
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
		// target scopes.
		decl := t.Obj()
		if !decl.Exported() || !o.scopes[decl.Parent()] {
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

// Union constructs a synthetic union-interface type.
func (o *Oracle) Union(pkg *types.Package, name string, reachable bool) *Union {
	if o.union != nil {
		return o.union
	}
	o.union = &Union{
		decl:      types.NewTypeName(token.NoPos, pkg, name, types.NewInterfaceType(nil, nil)),
		reachable: reachable,
		name:      name,
	}
	return o.union
}

// VisitableFrom returns the declared types which should be considered
// visitable from any of the declared seed types. Whether or not a type
// is visitable is approximately equal to asking whether or not it is
// assignable, with the caveat that this method will consider pointer
// receivers for interface types. There is also special handling for
// a synthetic union interface (represented by *Union).
func (o *Oracle) VisitableFrom(seeds TraversableSet) TraversableSet {
	seedSet := make(map[types.Object]bool)
	intfs := make(map[*types.Interface]bool)
	isReachable := false

	for s := range seeds {
		if u, ok := s.(*Union); ok {
			isReachable = isReachable || u.reachable
		}
		if decl := s.Declaration(); decl != nil {
			seedSet[decl] = true
			if intf, ok := decl.Type().Underlying().(*types.Interface); ok {
				intfs[intf] = true
			}
		}
	}

	ret := NewTraversableSet()
	for _, t := range o.data {
		if t == nil {
			continue
		}
		decl := t.Declaration()
		if decl == nil {
			continue
		}
		if isReachable {
			ret.Add(t)
		} else if seedSet[decl] {
			ret.Add(t)
		} else {
			for intf := range intfs {
				if types.Implements(decl.Type(), intf) {
					ret.Add(t)
					break
				} else if ptr := types.NewPointer(decl.Type()); types.Implements(ptr, intf) {
					found, _ := o.Get(ptr)
					ret.Add(found)
					break
				}
			}
		}
	}

	return ret
}
