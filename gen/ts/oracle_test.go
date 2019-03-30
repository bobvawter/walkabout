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

package ts_test

import (
	"go/types"
	"sort"
	"testing"

	"github.com/cockroachdb/walkabout/gen/ts"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

type testCase struct {
	ActionableFields []string
	Elem             *testCase
	FieldChecks      map[string]*testCase
	Name             string
	Kind             ts.Kind
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
	o := ts.NewOracle([]*types.Scope{pkg.Scope()})

	found := make(map[string]*ts.T)
	for _, t := range o.All() {
		if decl := t.Declaration(); decl != nil {
			found[decl.Name()] = t
		}
	}

	tcs := map[string]*testCase{
		"ByRefType": {
			Kind: ts.Struct,
		},
		"ByValType": {
			Kind: ts.Struct,
		},
		"ContainerType": {
			ActionableFields: []string{
				"AnotherTarget", "AnotherTargetPtr", "ByRef", "ByRefPtr", "ByRefPtrSlice", "ByRefSlice",
				"ByVal", "ByValPtr", "ByValPtrSlice", "ByValSlice", "Container", "EmbedsTarget",
				"EmbedsTargetPtr", "InterfacePtrSlice", "NamedTargets", "ReachableType", "TargetSlice",
				"UnionableType",
			},
			FieldChecks: map[string]*testCase{
				"ByVal": {
					Kind: ts.Struct,
					Name: "ByValType",
				},
				"ByValPtr": {
					Kind: ts.Pointer,
					Elem: &testCase{
						Name: "ByValType",
						Kind: ts.Struct,
					},
				},
				"ByValPtrSlice": {
					Kind: ts.Slice,
					Elem: &testCase{
						Kind: ts.Pointer,
						Elem: &testCase{
							Name: "ByValType",
							Kind: ts.Struct,
						},
					},
				},
				"NamedTargets": {
					Kind: ts.Slice,
					Name: "Targets",
					Elem: &testCase{
						Name: "Target",
						Kind: ts.Interface,
					},
				},
			},
			Kind: ts.Struct,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			a := assert.New(t)
			tc.Name = name
			check(a, tc, found[name])
		})
	}
}

func check(a *assert.Assertions, tc *testCase, v *ts.T) {
	if !a.NotNil(v, "not found") {
		return
	}

	if decl := v.Declaration(); decl != nil {
		a.Equal(tc.Name, decl.Name())
	}

	if tc.Kind != 0 && a.NotNil(v.Traversable()) {
		a.Equal(tc.Kind, v.Traversable().Kind())

		switch v.Traversable().Kind() {
		case ts.Pointer, ts.Slice:
			if tc.Elem != nil {
				check(a, tc.Elem, v.Traversable().Elem())
			}

		case ts.Struct:
			var names []string
			for _, field := range v.Traversable().Fields() {
				names = append(names, field.Name())
			}
			sort.Strings(names)
			a.Equal(tc.ActionableFields, names)

			fieldsByName := make(map[string]*ts.T, len(v.Traversable().Fields()))
			for _, field := range v.Traversable().Fields() {
				fieldsByName[field.Name()] = field.Type()
			}

			for name, sub := range tc.FieldChecks {
				if field := fieldsByName[name]; a.NotNil(field, name) {
					check(a, sub, field)
				}
			}
		}
	}
}
