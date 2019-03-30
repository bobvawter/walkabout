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

func TestNewView(t *testing.T) {
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

	v := view.NewView(o, target)

	checkVisitable(a, v, "*ByRefType", "*ContainerType", "ByValType", "EmbedsTarget", "Target", "Targets")

	checkImplements(a, v, target,
		"*ByRefType", "*ContainerType", "ByValType", "EmbedsTarget", "Target", "Targets")
	checkImplements(a, v, o.Get(pkg.Scope().Lookup("EmbedsTarget").Type()),
		"ByValType", "EmbedsTarget")

	checkTraversable(a, v,
		"*ByRefType", "*ByValType", "*ContainerType", "*EmbedsTarget", "ByValType", "ContainerType",
		"EmbedsTarget", "Target", "Targets", "[]*ByValType", "[]ByValType")
}

func checkImplements(a *assert.Assertions, v *view.View, intf *ts.T, expected ...string) {
	var names []string
	for _, impl := range v.Interfaces[intf] {
		names = append(names, impl.String())
	}
	sort.Strings(expected)
	sort.Strings(names)
	a.Equal(expected, names)
}

func checkTraversable(a *assert.Assertions, v *view.View, expected ...string) {
	var names []string
	for impl := range v.Traversable {
		names = append(names, impl.String())
	}
	sort.Strings(expected)
	sort.Strings(names)
	a.Equal(expected, names)
}

func checkVisitable(a *assert.Assertions, v *view.View, expected ...string) {
	var names []string
	for visitable := range v.Visitable {
		names = append(names, visitable.String())
	}
	sort.Strings(expected)
	sort.Strings(names)
	a.Equal(expected, names)
}
