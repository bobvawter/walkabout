// Copyright 2018 The Cockroach Authors.
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

package gen

import (
	"go/token"
	"go/types"
	"io"
	"os"
	"strings"

	"github.com/cockroachdb/walkabout/gen/ts"
	"github.com/cockroachdb/walkabout/gen/view"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

type config struct {
	dir string
	// If present, overrides the output file name.
	outFile string
	// Include all types reachable from visitable types that implement
	// the root visitable interface.
	reachable bool
	// The requested type names.
	typeNames []string
	// If present, unifies all specified interfaces under a single
	// visitable interface with this name.
	union string
}

// generation represents an entire run of the code generator. The
// overall flow is broken up into various stages, which can be seen in
// Execute().
type generation struct {
	config

	// Allows additional files to be added to the parse phase for testing.
	extraTestSource map[string][]byte
	fileSet         token.FileSet
	writeCloser     func(name string) (io.WriteCloser, error)
}

// newGeneration constructs a generation which will look for the
// named interface types in the given directory.
func newGeneration(cfg config) (*generation, error) {
	if len(cfg.typeNames) > 1 && cfg.union == "" {
		return nil, errors.New("multiple input types can only be used with --union")
	}
	if cfg.reachable && cfg.union == "" {
		return nil, errors.New("--reachable can only be used with --union")
	}
	return &generation{
		config: cfg,
		writeCloser: func(name string) (io.WriteCloser, error) {
			if name == "-" {
				return os.Stdout, nil
			}
			return os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		},
	}, nil
}

// Execute runs the complete code-generation cycle.
func (g *generation) Execute() error {
	// This will return multiple packages.Package if we're also loading
	// test files. Note that the error here is whether or not the Load()
	// was able to perform its work. The underlying source may still have
	// syntax/type errors, but we ignore that in case of a "make clean"
	// situation, where we're likely to see code that depends on generated
	// code.
	pkgs, err := packages.Load(g.packageConfig(), ".")
	if err != nil {
		return err
	}

	// Gather the namespace scopes that we'll resolve symbols in. We're
	// also on the lookout for symbols defined in test files so we know to
	// emit a test suffix.
	scopes := make([]*types.Scope, len(pkgs))
	test := false
	for i, pkg := range pkgs {
		scope := pkg.Types.Scope()
		scopes[i] = scope
		pos := g.fileSet.Position(scope.Pos())
		if strings.HasSuffix(pos.Filename, "_test.go") {
			test = true
		}
	}

	// Synthesize a fake type declaration for the union interface.
	if g.union != "" {
		unionType := types.NewNamed(
			types.NewTypeName(token.NoPos, pkgs[0].Types, g.union, nil),
			types.NewInterfaceType(nil, nil), nil)
		if scopes[0].Insert(unionType.Obj()) != nil {
			return errors.Errorf("a type named %q already exists", g.union)
		}
	}

	// Initialize the type collection.
	o := ts.NewOracle(scopes)

	var root *ts.T
	if g.union != "" {
		root = o.Get(scopes[0].Lookup(g.union).Type())
	}

	// Locate the seed types.
	var seeds []*ts.T
name:
	for _, name := range g.typeNames {
		for _, scope := range scopes {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}
			found := o.Get(obj.Type())
			seeds = append(seeds, found)
			if root == nil {
				root = found
			}
			continue name
		}
		return errors.Errorf("unknown type %q", name)
	}

	// Compute the type relationships that we care about.
	v := view.New(o, seeds, g.reachable)

	// Generate code!
	return g.generateAPI(root, v, test, g.union != "")
}

func (g *generation) packageConfig() *packages.Config {
	return &packages.Config{
		Dir:     g.dir,
		Fset:    &g.fileSet,
		Mode:    packages.LoadTypes,
		Overlay: g.extraTestSource,
		Tests:   true,
	}
}
