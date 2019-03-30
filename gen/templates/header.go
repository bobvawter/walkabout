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

// Package templates contains the walkabout code templates.
package templates

// TemplateSources contains the templates to aggregate.
var TemplateSources = make(map[string]string)

func init() {
	TemplateSources["00header"] = `
// Code generated by github.com/cockroachdb/walkabout. DO NOT EDIT.

package {{ Package }}

import (
	"fmt"
	"unsafe"

	e "github.com/cockroachdb/walkabout/engine"
)
`
}
