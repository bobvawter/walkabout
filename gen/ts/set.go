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

// TraversableSet is a set of Traversable objects.
type TraversableSet map[Traversable]struct{}

// NewTraversableSet constructs a set containing the given values.
func NewTraversableSet(values ...Traversable) TraversableSet {
	ret := make(TraversableSet, len(values))
	for _, v := range values {
		ret[v] = struct{}{}
	}
	return ret
}

// Add ensures that the set contains the given type.
func (s TraversableSet) Add(t Traversable) {
	s[t] = struct{}{}
}

// Contains returns true if the type is present in the set.
func (s TraversableSet) Contains(t Traversable) bool {
	_, found := s[t]
	return found
}

// ShouldTraverse returns true if any of the set's types
// are reachable from the given origin type. This function does not
// take assignability into account, so the set of visitable types
// must be complete.
func (s TraversableSet) ShouldTraverse(origin Traversable) bool {
	return s.shouldTraverse(make(TraversableSet), origin)
}

func (s TraversableSet) shouldTraverse(seen TraversableSet, origin Traversable) bool {
	if _, ok := s[origin]; ok {
		return true
	}

	// Prevent cycles.
	if seen.Contains(origin) {
		return false
	}
	seen.Add(origin)

	switch t := origin.(type) {
	case Elementary:
		// Handles Field, Pointer, and Slice
		return s.shouldTraverse(seen, t.Elem())
	case *Struct:
		for _, f := range t.fields {
			if s.shouldTraverse(seen, f) {
				return true
			}
		}
	}
	return false
}
