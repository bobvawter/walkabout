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

package templates

func init() {
	TemplateSources["10api"] = `
{{- $v := . -}}
{{- $Abstract := T $v "Abstract" -}}
{{- $Action := T $v "Action" -}}
{{- $Context := T $v "Context" -}}
{{- $Decision := T $v "Decision" -}}
{{- $Root := $v.Root -}}
{{- $TypeId := T $v "TypeId " -}}
{{- $WalkerFn := T $v "WalkerFn" -}}

// {{ $TypeId }} is a lightweight type token.
type {{ $TypeId }} e.TypeId

// {{ $Abstract }} allows users to treat a {{ $Root }} as an abstract
// tree of nodes. All visitable struct types will have generated methods
// which implement this interface. 
type {{ $Abstract }} interface {
	// ChildAt returns the nth field of a struct or nth element of a
	// slice. If the child is a type which directly implements
	// {{ $Abstract }}, it will be returned. If the child is of a pointer or
	// interface type, the value will be automatically dereferenced if it
	// is non-nil. If the child is a slice type, a {{ $Abstract }} wrapper
	// around the slice will be returned.
	ChildAt(index int) {{ $Abstract }}
	// NumChildren returns the number of visitable fields in a struct,
	// or the length of a slice.
	NumChildren() int
	// TypeId returns a type token.
	TypeId() {{ $TypeId }}
}

var (
{{- range $s := Structs $v -}}
_ {{ $Abstract }} = &{{ $s }}{};
{{- end -}}
)

// {{ $WalkerFn }} is used to implement a visitor pattern over
// types which implement {{ $Root }}.
//
// Implementations of this function return a {{ $Decision }}, which
// allows the function to control traversal. The zero value of
// {{ $Decision }} means "continue". Other values can be obtained from the
// provided {{ $Context }} to stop or to return an error.
//
// A {{ $Decision }} can also specify a post-visit function to execute
// or can be used to replace the value being visited.
type {{ $WalkerFn }} func(ctx {{ $Context }}, x {{ $Root }}) {{ $Decision }}

// {{ $Context }} is provided to {{ $WalkerFn }} and acts as a factory
// for constructing {{ $Decision }} instances.
type {{ $Context }} struct {
	impl e.ContextImpl
}

// Continue returns the zero-value of {{ $Decision }}. It exists only
// for cases where it improves the readability of code.
func (c *{{ $Context }}) Continue() {{ $Decision }} {
	return {{ $Decision }}{}
}

// Error returns a {{ $Decision }} which will cause the given error
// to be returned from the Walk() function. Post-visit functions
// will not be called.
func (c *{{ $Context }}) Error(err error) {{ $Decision }} {
	return {{ $Decision }}{impl: e.DecisionImpl{Error: err}}
}

// Halt will end a visitation early and return from the Walk() function.
// Any registered post-visit functions will be called.
func (c *{{ $Context }}) Halt() {{ $Decision }} {
	return {{ $Decision }}{impl: e.DecisionImpl{Halt: true}}
}

// Actions will perform the given actions in place of visiting values
// that would normally be visited.  This allows callers to control
// specific field visitation order or to insert additional callbacks
// between visiting certain values.
func (c *{{ $Context }}) Actions(actions ...{{ $Action }}) {{ $Decision }} {
	if actions == nil || len(actions) == 0 {
		return c.Skip()
	}

	ret := make([]e.ActionImpl, len(actions))
	for i, a := range actions {
		ret[i] = e.ActionImpl(a)
	}

	return {{ $Decision }} { impl: e.DecisionImpl { Actions: ret } }
}

// Skip will not traverse the fields of the current object.
func (c *{{ $Context }}) Skip() {{ $Decision }} {
	return {{ $Decision }}{impl: e.DecisionImpl{Skip: true}}
}

// {{ $Decision }} is used by {{ $WalkerFn }} to control visitation.
// The {{ $Context }} provided to a {{ $WalkerFn }} acts as a factory
// for {{ $Decision }} instances. In general, the factory methods
// choose a traversal strategy and additional methods on the
// {{ $Decision }} can achieve a variety of side-effects.
type {{ $Decision }} struct {
	impl e.DecisionImpl
}

// Intercept registers a function to be called immediately before 
// visiting each field or element of the current value.
func (d {{ $Decision }}) Intercept(fn {{ $WalkerFn }}) {{ $Decision }} {
	d.impl.Intercept = fn
	return d
}

// Post registers a post-visit function, which will be called after the
// fields of the current object. The function can make another decision
// about the current value.
func (d {{ $Decision }}) Post(fn {{ $WalkerFn }}) {{ $Decision }} {
	d.impl.Post = fn
	return d
}

// Replace allows the currently-visited value to be replaced. All
// parent nodes will be cloned.
func (d {{ $Decision }}) Replace(x {{ $Root }}) {{ $Decision }} {
	switch t := x.(type) {
		{{ range $imp := Implementors $Root -}}
		case {{ $imp.Actual }}:
			d.impl.ReplacementType = e.TypeId({{ TypeId $imp.Underlying }});
			{{ if IsPointer $imp.Actual }}d.impl.Replacement = e.Ptr(t);
			{{ else }}d.impl.Replacement = e.Ptr(&t);
			{{ end }}
		{{- end -}}
		default:
			panic("unhandled type passed to Replace(). Is the generated code out of date?")
	}
	return d
}


// {{ $Action }} is used by {{ $Context }}.Actions() and allows users
// to have fine-grained control over traversal.
type {{ $Action }} e.ActionImpl

// ActionVisit constructs a {{ $Action }} that will visit the given value.
func (*{{ $Context }}) ActionVisit(x {{ $Root }}) {{ $Action }} {
	d := {{ $Decision }}{}.Replace(x)
	return {{ $Action }} (e.ActionVisitTypeId(d.impl.ReplacementType, d.impl.Replacement))
}

// ActionCall constructs a {{ $Action }} that will invoke the given callback.
func (*{{ $Context }}) ActionCall(fn func()error) {{ $Action }} {
	return {{ $Action }} (e.ActionCall(fn))
}
`
}
