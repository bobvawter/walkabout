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
	// the concrete, visitable types which implement it.
	Interfaces map[*ts.T][]*ts.T
	// This map contains all types which should be passed to the user's
	// callback function.
	Visitable map[*ts.T]types.Object
	// This map contains all objects for which we have a traversal
	// strategy that will lead to one of the visitable types.
	Traversable map[*ts.T]*ts.Traversable
}

// New constructs a view around some number of seed types.
func New(o *ts.Oracle, seeds []*ts.T, allReachable bool) *View {
	ret := &View{
		Interfaces:  make(map[*ts.T][]*ts.T),
		Visitable:   make(map[*ts.T]types.Object),
		Traversable: make(map[*ts.T]*ts.Traversable),
	}

	// In the first pass will extract all types that we're going to
	// consider to be visitable.
	for _, t := range o.All() {
		for _, seed := range seeds {
			if as, obj, ok := shouldVisitFrom(seed, t); ok {
				ret.Visitable[as] = obj
			}
		}
		// Identify interface types while we're here.
		if decl, tr := t.Declaration(), t.Traversable(); decl != nil &&
			tr != nil && tr.Kind() == ts.Interface {
			ret.Interfaces[t] = nil
		}
	}

	// Inflate the interfaces map, choosing from the types that we know
	// to be visitable.
	for intf, slice := range ret.Interfaces {
		for visitable := range ret.Visitable {
			if as, _, ok := shouldVisitFrom(intf, visitable); ok {
				if as.Traversable().Kind() == ts.Interface {
					continue
				}
				slice = append(slice, as)
				if as.Traversable().Kind() == ts.Struct {
					slice = append(slice, as.PointerTo())
				}
			}
		}
		ret.Interfaces[intf] = slice
	}

	// Now that we know all of the types that we want to present to the
	// user, we chase traversal strategies that lead to a visitable type. We
	// do this by first building up a map of implied visitability and
	// then flattening it out into the final presentation.
	implies := make(map[*ts.T][]*ts.T, len(ret.Visitable))
	seen := make(map[*ts.T]bool)
	for visitable := range ret.Visitable {
		ret.chaseTraversable(visitable, implies, seen)
	}

	for visitable := range ret.Visitable {
		ret.unpackTraversable(visitable, allReachable, implies)
	}

	return ret
}

// chaseTraversable computes implied traversability.  The values of
// the map will contain the types that are traversable if the key is
// traversable.
func (v *View) chaseTraversable(t *ts.T, implies map[*ts.T][]*ts.T, seen map[*ts.T]bool) {
	// Break cycles.
	if seen[t] {
		return
	}
	seen[t] = true

	// If the type isn't Traversable, we can ignore it.
	tr := t.Traversable()
	if tr == nil {
		return
	}

	add := func(key *ts.T) {
		implies[key] = append(implies[key], t)
		v.chaseTraversable(key, implies, seen)
	}

	switch tr.Kind() {
	case ts.Slice, ts.Pointer:
		// Simple things with an element.
		add(tr.Elem())

	case ts.Interface:
		// Need to pick off all types which implement the interface.
		for _, impl := range v.Interfaces[t] {
			add(impl)
		}

	case ts.Struct:
		// Evaluate all fields.
		for _, field := range tr.Fields() {
			add(field.Type())
		}

	default:
		panic(errors.Errorf("unimplemented: %v", tr.Kind()))
	}
}

func (v *View) unpackTraversable(t *ts.T, allReachable bool, implies map[*ts.T][]*ts.T) {
	if _, done := v.Traversable[t]; done {
		return
	}
	tr := t.Traversable()
	v.Traversable[t] = tr

	// Add directly implied types.
	for _, next := range implies[t] {
		v.unpackTraversable(next, allReachable, implies)
	}

	// Add implicitly implied types. For instance *Foo requires Foo.
	switch t.Traversable().Kind() {
	case ts.Slice, ts.Pointer:
		// Simple things with an element.
		v.unpackTraversable(tr.Elem(), allReachable, implies)

	case ts.Interface:
		// Interfaces don't actually imply anything.

	case ts.Struct:
		// Include all struct fields if requested.
		if allReachable {
			for _, field := range tr.Fields() {
				v.unpackTraversable(field.Type(), allReachable, implies)
			}
		}

	default:
		panic(errors.Errorf("unimplemented: %v", tr.Kind()))
	}
}

// shouldVisitFrom embeds our idea of visitability.  This is very
// similar to asking whether or not the target type is assignable to the
// seed type, except that we'll fudge things to account for pointer vs
// value receivers when dealing with interfaces.
func shouldVisitFrom(seed, target *ts.T) (*ts.T, types.Object, bool) {
	seedDecl := seed.Declaration()
	if seedDecl == nil {
		return nil, nil, false
	}
	if seed == target {
		return seed, seedDecl, true
	}
	seedIntf, _ := seedDecl.Type().Underlying().(*types.Interface)
	if seedIntf == nil {
		return nil, nil, false
	}

	if targetDecl := target.Declaration(); targetDecl != nil {
		if types.Implements(targetDecl.Type(), seedIntf) {
			return target, targetDecl, true
		}
		if types.Implements(types.NewPointer(targetDecl.Type()), seedIntf) {
			return target.PointerTo(), targetDecl, true
		}
	} else if tr := target.Traversable(); tr.Kind() == ts.Pointer {
		return shouldVisitFrom(seed, tr.Elem())
	}
	return nil, nil, false
}
