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
	"bytes"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

var configs = map[string]config{
	"single": {
		dir:       "../demo",
		typeNames: []string{"Target"},
	},
	"union": {
		dir:       "../demo",
		typeNames: []string{"Target", "Unionable"},
		union:     "Union",
	},
	"unionReachable": {
		dir:       "../demo",
		typeNames: []string{"Target", "Unionable"},
		union:     "Union",
		reachable: true},
	"structUnion": {
		dir:       "../demo",
		typeNames: []string{"ContainerType", "ByValType"},
		union:     "Union"},
	"structUnionReachable": {
		dir:       "../demo",
		typeNames: []string{"ContainerType"},
		union:     "Union",
		reachable: true},
}

// Verify that our example data in the demo package is correct and
// that we won't break the existing test code with updated outputs.
// This test has two phases.  The first generates the code we want
// to emit and the second performs a complete type-checking of the
// demo package to make sure that any changes to the generated
// code will compile.
func TestExampleData(t *testing.T) {
	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			a := assert.New(t)
			outputs := make(map[string][]byte)

			newGeneration := func() (*generation, string, error) {
				g, err := newGenerationForTesting(cfg, outputs)
				if err != nil {
					return nil, "", err
				}
				if cfg.union == "" {
					return g, cfg.typeNames[0], nil
				}
				return g, cfg.union, nil
			}

			g, _, err := newGeneration()
			if !a.NoError(err) {
				return
			}

			if !a.NoError(g.Execute()) {
				for k, v := range outputs {
					t.Logf("%s\n%s\n\n\n", k, string(v))
				}
				return
			}

			cfg := g.packageConfig()
			cfg.Mode = packages.LoadAllSyntax
			cfg.Overlay = outputs

			pkgs, err := packages.Load(cfg, ".")
			if a.NoError(err) {
				for _, pkg := range pkgs {
					a.Nil(pkg.Errors)
				}
			}
		})
	}
}

// Run the generator twice to ensure that it produces stable output.
func TestOutputIsStable(t *testing.T) {
	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			a := assert.New(t)

			outputs1 := make(map[string][]byte)
			g1, err := newGenerationForTesting(cfg, outputs1)
			if !a.NoError(err) {
				return
			}
			a.NoError(g1.Execute())
			a.True(len(outputs1) > 0, "no outputs")

			outputs2 := make(map[string][]byte)
			g2, err := newGenerationForTesting(cfg, outputs2)
			if !a.NoError(err) {
				return
			}
			a.NoError(g2.Execute())
			a.True(len(outputs2) > 0, "no outputs")

			a.Equal(outputs1, outputs2)
		})
	}
}

// newGenerationForTesting creates a generator that captures
// its output in the provided map.
func newGenerationForTesting(cfg config, outputs map[string][]byte) (*generation, error) {
	g, err := newGeneration(cfg)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	g.writeCloser = func(name string) (io.WriteCloser, error) {
		// Use absolute filenames for compatibility with package overlay.
		name, err := filepath.Abs(name)
		if err != nil {
			return nil, err
		}
		return newMapWriter(name, &mu, outputs), nil
	}
	return g, nil
}

// mapWriter is a trivial implementation of io.WriteCloser that captures
// its output in a map. Access to the map is synchronized via a
// shared mutex.
type mapWriter struct {
	buf  bytes.Buffer
	name string
	mu   struct {
		*sync.Mutex
		dest map[string][]byte
	}
}

func newMapWriter(name string, mu *sync.Mutex, outputs map[string][]byte) io.WriteCloser {
	ret := &mapWriter{name: name}
	ret.mu.Mutex = mu
	ret.mu.dest = outputs
	return ret
}

// Write implements io.Writer.
func (w *mapWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// Close implements io.Closer.
func (w *mapWriter) Close() error {
	w.mu.Lock()
	if w.mu.dest != nil {
		w.mu.dest[w.name] = w.buf.Bytes()
	}
	w.mu.Unlock()
	return nil
}
