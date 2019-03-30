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
)

// Oracle holds typesystem information.
type Oracle struct {
	// We store nil values if a type isn't handled.
	data map[types.Type]*T
	// We limit Traversable creation to types defined within these scopes.
	scopes map[*types.Scope]bool
}

// NewOracle constructs a new Oracle instance from the types defined
// within the given package.
func NewOracle(scopes []*types.Scope) *Oracle {
	scopeMap := make(map[*types.Scope]bool, len(scopes))
	o := &Oracle{
		data:   make(map[types.Type]*T),
		scopes: scopeMap,
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

// All returns all types known to the Oracle.
func (o *Oracle) All() []*T {
	ret := make([]*T, 0, len(o.data))
	for _, v := range o.data {
		if v.IsIgnored() {
			continue
		}
		ret = append(ret, v)
		if v.pointerTo != nil {
			ret = append(ret, v.pointerTo)
		}
		if v.sliceOf != nil {
			ret = append(ret, v.sliceOf)
		}
	}
	return ret
}

// Get creates or returns a Traversable that hold extracted information
// about the given type.
func (o *Oracle) Get(typ types.Type) *T {
	// Simple case.
	if found, ok := o.data[typ]; ok {
		return found
	}

	switch t := typ.(type) {
	case *types.Pointer:
		// This is a hack to improve developer convenience. When we get
		// instances of types.Type from the type-checker, they've all been
		// canonicalized, so we can use pointer references for identity.  We
		// perform a similar canonicalization so that a caller can do the
		// obvious thing and call Oracle.Get(types.NewPointer(someType)) and
		// be guaranteed to receive the same instance each time.
		return o.Get(t.Elem()).PointerTo()
	case *types.Slice:
		return o.Get(t.Elem()).SliceOf()

	case *types.Named:
		// We only want to consider exported types that are defined within the
		// target scopes.
		decl := t.Obj()
		if !decl.Exported() || !o.scopes[decl.Parent()] {
			o.data[typ] = ignoredT
			return ignoredT
		}
		ret := &T{declaration: decl}
		o.data[typ] = ret

		under := o.Get(decl.Type().Underlying())
		ret.traversable = under.traversable
		return ret

	case *types.Interface:
		ret := &T{traversable: &Traversable{kind: Interface}}
		o.data[typ] = ret
		return ret

	case *types.Struct:
		ret := &T{}
		o.data[typ] = ret

		fields := make([]*Field, 0, t.NumFields())
		for i, j := 0, t.NumFields(); i < j; i++ {
			f := t.Field(i)
			if !f.Exported() {
				continue
			}
			if field := o.Get(f.Type()); !field.IsIgnored() {
				fields = append(fields, &Field{name: f.Name(), typ: field})
			}
		}
		ret.traversable = &Traversable{
			fields: fields,
			kind:   Struct,
		}
		return ret

	default:
		o.data[typ] = ignoredT
		return ignoredT
	}
}
