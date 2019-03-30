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

package view_test

import (
	"go/types"
	"sort"
	"testing"

	"github.com/cockroachdb/walkabout/gen/ts"
	"github.com/cockroachdb/walkabout/gen/view"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestView(t *testing.T) {
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

	target := o.Get(pkg.Scope().Lookup("Target").Type())
	if !a.NotNil(target) || !a.False(target.IsIgnored()) {
		return
	}

	tcs := map[string]struct {
		AllReachable bool
		Seeds        []string
		Implements   map[string][]string
		Traversable  []string
		Visitable    []string
	}{
		"empty": {},
		"target": {
			Seeds: []string{"Target"},
			Implements: map[string][]string{
				"EmbedsTarget": {"*ByValType", "ByValType"},
				"Target": {
					"*ByRefType", "*ByValType", "*ContainerType", "ByValType", "Targets",
				},
			},
			Traversable: []string{
				"*ByRefType", "*ByValType", "*ContainerType", "*EmbedsTarget",
				"*Target", "ByRefType", "ByValType", "ContainerType",
				"EmbedsTarget", "Target", "Targets", "[]*ByRefType",
				"[]*ByValType", "[]*Target", "[]ByRefType", "[]ByValType",
				"[]Target",
			},
			Visitable: []string{
				"*ByRefType", "*ContainerType", "ByValType", "EmbedsTarget",
				"Target", "Targets",
			},
		},
		"justStructs": {
			Seeds: []string{"ByValType", "ByRefType", "ContainerType"},
			Traversable: []string{
				"*ByRefType", "*ByValType", "*ContainerType", "*EmbedsTarget",
				"*Target", "ByRefType", "ByValType", "ContainerType",
				"EmbedsTarget", "Target", "Targets", "[]*ByRefType",
				"[]*ByValType", "[]*Target", "[]ByRefType", "[]ByValType",
				"[]Target",
			},
			Visitable: []string{"ByRefType", "ByValType", "ContainerType"},
		},
		"single": {
			Seeds: []string{"ContainerType"},
			Traversable: []string{
				"*ContainerType", "*Target", "ContainerType", "Target",
				"Targets", "[]*Target", "[]Target",
			},
			Visitable: []string{"ContainerType"},
		},
		"targetUnion": {
			AllReachable: true,
			Seeds:        []string{"Target"},
			Implements: map[string][]string{
				"EmbedsTarget": {"*ByValType", "ByValType"},
				"Target": {
					"*ByRefType", "*ByValType", "*ContainerType", "ByValType", "Targets",
				},
			},
			Traversable: []string{
				"*ByRefType", "*ByValType", "*ContainerType", "*EmbedsTarget",
				"*Target", "*UnionableType", "ByRefType", "ByValType",
				"ContainerType", "EmbedsTarget", "ReachableType", "Target",
				"Targets", "UnionableType", "[]*ByRefType", "[]*ByValType",
				"[]*Target", "[]ByRefType", "[]ByValType", "[]Target",
			},
			Visitable: []string{
				"*ByRefType", "*ContainerType", "ByValType", "EmbedsTarget",
				"Target", "Targets",
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			a := assert.New(t)

			seeds := make([]*ts.T, 0, len(tc.Seeds))
			for _, seed := range tc.Seeds {
				t := o.Get(pkg.Scope().Lookup(seed).Type())
				seeds = append(seeds, t)
			}
			v := view.New(o, seeds, tc.AllReachable)

			for intfName, impls := range tc.Implements {
				t := o.Get(pkg.Scope().Lookup(intfName).Type())
				checkImplements(a, v, t, impls)
			}
			checkTraversable(a, v, tc.Traversable)
			checkVisitable(a, v, tc.Visitable)
		})
	}
}

func checkImplements(a *assert.Assertions, v *view.View, intf *ts.T, expected []string) {
	var names []string
	for _, impl := range v.Interfaces[intf] {
		names = append(names, impl.String())
	}
	sort.Strings(expected)
	sort.Strings(names)
	a.Equal(expected, names, "implements")
	a.NotContains(names, "NeverType")
}

func checkTraversable(a *assert.Assertions, v *view.View, expected []string) {
	var names []string
	for impl := range v.Traversable {
		names = append(names, impl.String())
	}
	sort.Strings(expected)
	sort.Strings(names)
	a.Equal(expected, names, "traversable")
	a.NotContains(names, "NeverType")
}

func checkVisitable(a *assert.Assertions, v *view.View, expected []string) {
	var names []string
	for visitable := range v.Visitable {
		names = append(names, visitable.String())
	}
	sort.Strings(expected)
	sort.Strings(names)
	a.Equal(expected, names, "visitable")
	a.NotContains(names, "NeverType")
}
