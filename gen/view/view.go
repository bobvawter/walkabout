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

// Package view extracts information from a ts.Oracle into formats
// more easily consumed by the template code.
package view

import (
	"go/types"

	"github.com/pkg/errors"

	"github.com/cockroachdb/walkabout/gen/ts"
)

// A View provides filtering of the contents of a type oracle based
type View struct {
	// Interfaces contains a mapping of every visitable interface type to
	// the visitable types which implement it.
	Interfaces map[*ts.T][]*ts.T
	// This map contains all types which should be passed to the user's
	// callback function.
	Visitable map[*ts.T]types.Object
	// This map contains all objects for which we have a traversal
	// strategy that will lead to one of the visitable types.
	Traversable map[*ts.T]*ts.Traversable
}

func NewView(o *ts.Oracle, seed *ts.T) *View {
	ret := &View{
		Interfaces:  make(map[*ts.T][]*ts.T),
		Visitable:   make(map[*ts.T]types.Object),
		Traversable: make(map[*ts.T]*ts.Traversable),
	}

	for _, t := range o.All() {
		if decl := t.Declaration(); decl != nil {
			if as, ok := shouldVisitFrom(seed, t); ok {
				ret.Visitable[as] = decl

				// See the traversal strategies for the visitable types.
				if tr := t.Traversable(); tr != nil {
					if tr.Kind() == ts.Interface {
						ret.Interfaces[t] = nil
					}

					ret.Traversable[as] = tr
				}
			}
		}
	}

	// Inflate the interfaces map.
	for intf, slice := range ret.Interfaces {
		for visitable := range ret.Visitable {
			if as, ok := shouldVisitFrom(intf, visitable); ok {
				slice = append(slice, as)
			}
		}
		ret.Interfaces[intf] = slice
	}

	// Now that we know all of the types that we want to present to the
	// user, we chase strategies that lead to a visitable type.
	temp := make(map[*ts.T]bool, len(ret.Visitable))
	for visitable := range ret.Visitable {
		chaseTraversable(ret, visitable, temp)
	}
	for t, ok := range temp {
		if ok {
			ret.Traversable[t] = t.Traversable()
		}
	}

	return ret
}

// chaseTraversable looks for types which are in the Visitable map,
// or which will eventually lead to a Visitable type.
func chaseTraversable(v *View, t *ts.T, into map[*ts.T]bool) bool {
	// We may have stored a false in the map in order to prevent cycles.
	if ret, found := into[t]; found {
		return ret
	}

	// Break cycles. We may override this later.
	into[t] = false

	// If the type isn't Traversable, we can ignore it.
	tr := t.Traversable()
	if tr == nil {
		return false
	}

	// Is the type visitable?
	_, ret := v.Visitable[t]

	switch tr.Kind() {
	case ts.Field, ts.Slice, ts.Pointer:
		// Simple things with an element.
		ret = chaseTraversable(v, tr.Elem(), into)

	case ts.Interface:
		// Need to pick off all types which implement the interface.
		for _, impl := range v.Interfaces[t] {
			ret = chaseTraversable(v, impl, into) || ret
		}

	case ts.Struct:
		// Evaluate all fields.
		for _, field := range tr.Fields() {
			ret = chaseTraversable(v, field, into) || ret
		}

	default:
		panic(errors.Errorf("unimplemented: %v", tr.Kind()))
	}

	into[t] = ret
	return ret
}

func shouldVisitFrom(seed, target *ts.T) (*ts.T, bool) {
	if seed == target {
		return target, true
	}

	seedDecl := seed.Declaration()
	if seedDecl == nil {
		return nil, false
	}
	seedIntf, _ := seedDecl.Type().Underlying().(*types.Interface)
	if seedIntf == nil {
		return nil, false
	}

	targetDecl := target.Declaration()
	if targetDecl == nil {
		// If we're looking at a pointer, try unwrapping it.
		if tr := target.Traversable(); tr != nil && tr.Kind() == ts.Pointer {
			return shouldVisitFrom(seed, tr.Elem())
		}
		return nil, false
	}
	if types.Implements(targetDecl.Type(), seedIntf) {
		return target, true
	}
	if types.Implements(types.NewPointer(targetDecl.Type()), seedIntf) {
		return target.PointerTo(), true
	}
	return nil, false
}
