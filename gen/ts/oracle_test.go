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
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

type testCase struct {
	declName          string
	element           *testCase
	expectedType      Traversable
	found             Traversable
	totalFields       int
	traversableFields []string
}

// TestOracle will load our dummy source and verify that the typesystem
// is computed correctly.
func TestOracle(t *testing.T) {
	a := assert.New(t)

	cfg := &packages.Config{
		Dir:  "../../demo",
		Mode: packages.LoadTypes,
	}

	pkgs, err := packages.Load(cfg, ".")
	if !a.NoError(err) {
		return
	}
	a.Len(pkgs, 1)

	pkg := pkgs[0].Types
	o := NewOracle(pkg)

	var target *Interface
	for _, name := range pkg.Scope().Names() {
		if found, ok := o.Get(pkg.Scope().Lookup(name).Type()); ok {
			if name == "Target" {
				target = found.(*Interface)
			}
		}
	}
	if !a.NotNil(target) {
		return
	}

	visitable := o.VisitableFrom(target)

	tcs := map[string]*testCase{
		"*ByRefType": {
			expectedType: &Pointer{},
			element: &testCase{
				declName:     "ByRefType",
				expectedType: &Struct{},
				totalFields:  1,
			},
			totalFields: 1,
		},
		"*ContainerType": {
			expectedType: &Pointer{},
			element: &testCase{
				declName:     "ContainerType",
				expectedType: &Struct{},
				totalFields:  18,
				traversableFields: []string{
					"AnotherTarget", "AnotherTargetPtr", "ByRefPtr", "ByRefPtrSlice", "ByVal", "ByValPtr",
					"ByValPtrSlice", "ByValSlice", "Container", "EmbedsTarget", "EmbedsTargetPtr",
					"InterfacePtrSlice", "NamedTargets", "TargetSlice",
				},
			},
		},
		"ByValType": {
			declName:     "ByValType",
			expectedType: &Struct{},
			totalFields:  1,
		},
		"EmbedsTarget": {
			declName:     "EmbedsTarget",
			expectedType: &Interface{},
		},
		"Target": {
			declName:     "Target",
			expectedType: &Interface{},
		},
		"Targets": {
			declName:     "Targets",
			expectedType: &Slice{},
		},
	}
	tcs["Targets"].element = tcs["Target"]

	a.Len(visitable, len(tcs))

	for _, target := range visitable {
		key := QualifiedName("", target)
		if a.Contains(tcs, key) {
			tcs[key].found = target
		}
	}

	set := NewTraversableSet(visitable...)
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			a := assert.New(t)
			check(a, set, tc, tc.found)
		})
	}

	// Ensure that canonicalization works.
	s1, _ := o.Get(types.NewSlice(target.Declaration().Type()))
	s2, _ := o.Get(types.NewSlice(target.Declaration().Type()))
	a.Equal(s1, s2)

	p1, _ := o.Get(types.NewPointer(target.Declaration().Type()))
	p2, _ := o.Get(types.NewPointer(target.Declaration().Type()))
	a.Equal(p1, p2)
}

func check(
	a *assert.Assertions, visitable TraversableSet, expected *testCase, actual Traversable,
) {
	if expected.declName == "" {
		a.Nil(actual.Declaration())
	} else {
		a.Equal(expected.declName, actual.Declaration().Name())
	}

	if !a.IsType(expected.expectedType, actual) {
		return
	}

	switch t := actual.(type) {
	case Elementary:
		// This handles Field, Pointer, and Slice
		if a.NotNil(expected.element) {
			check(a, visitable, expected.element, t.Elem())
		}
	case *Struct:
		a.Len(t.fields, expected.totalFields)

		var traversableFields []string
		for _, f := range t.fields {
			if visitable.ShouldTraverse(f) {
				traversableFields = append(traversableFields, f.Declaration().Name())
			}
		}
		sort.Strings(traversableFields)

		a.Equal(expected.traversableFields, traversableFields)
	}
}
