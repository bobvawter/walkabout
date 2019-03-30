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
	TemplateSources["75typemap"] = `
{{- $Context := T "Context" -}}
{{- $Engine := t "Engine" -}}
{{- $TypeID := T "TypeID" -}}
{{- $WalkerFn := T "WalkerFn" -}}
// ------ Type Mapping ------
var {{ $Engine }} = e.New(e.TypeMap {
// ------ Structs ------
{{ range $s := Structs  }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) { *(*{{ $s }})(dest) = *(*{{ $s }})(from) },
	Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
		return e.Decision(fn.({{ $WalkerFn }})({{ $Context }}{impl}, (*{{ $s }})(x)))
	},
	Fields: []e.FieldInfo {
		{{ range $f := TraversableFields $s -}}
		{ Name: "{{ $f.Name }}", Offset: unsafe.Offsetof({{ $s }}{}.{{ $f.Name }}), Target: e.TypeID({{ TypeID $f.Type }})},
		{{ end }}
	},
	Name: "{{ $s }}",
	NewStruct: func() e.Ptr { return e.Ptr(&{{ $s }}{}) },
	SizeOf: unsafe.Sizeof({{ $s }}{}),
	Kind: e.KindStruct,
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Interfaces ------
{{ range $s := Interfaces }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	IntfType: func(x e.Ptr) e.TypeID {
		d := *(*{{ $s }})(x)
		switch d.(type) {
		{{ range $imp := ImplementorsOf $s -}}
		case {{ $imp }}:
			{{- if IsPointer $imp -}} 
				return e.TypeID({{ TypeID $imp.Traversable.Elem }});
			{{- else -}}
				return e.TypeID({{ TypeID $imp }});
			{{- end -}}
		{{- end }}
		default:
			return 0
		}
	},
	IntfWrap: func(id e.TypeID, x e.Ptr) e.Ptr {
		var d {{ $s }}
		switch {{ $TypeID }}(id) {
		{{ range $imp := ImplementorsOf $s -}}
			{{- if IsPointer $imp -}}
				case {{ TypeID $imp.Traversable.Elem }}: d = (*{{ $imp.Traversable.Elem }})(x);
				case {{ TypeID $imp }}: d = *(*{{ $imp }})(x);
			{{- end -}}
		{{- end }}
		default:
			return nil
		}
		return e.Ptr(&d)
	},
	Kind: e.KindInterface,
	Name: "{{ $s }}",
	SizeOf: unsafe.Sizeof({{ $s }}(nil)),
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Pointers ------
{{ range $s := Pointers }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Elem: e.TypeID({{ TypeID $s.Traversable.Elem }}),
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	Kind: e.KindPointer,
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Slices ------
{{ range $s := Slices }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Elem: e.TypeID({{ TypeID $s.Traversable.Elem }}),
	Kind: e.KindSlice,
	NewSlice: func(size int) e.Ptr {
		x := make({{ $s }}, size)
		return e.Ptr(&x)
	},
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
// ------ Opaque ------
{{ range $s := Opaques }}{{ TypeID $s }}: {
	Copy: func(dest, from e.Ptr) {
		*(*{{ $s }})(dest) = *(*{{ $s }})(from)
	},
	Kind: e.KindOpaque,
	Name: "{{ $s }}",
	SizeOf: unsafe.Sizeof(({{ $s }})(nil)),
	TypeID: e.TypeID({{ TypeID $s }}),
},
{{ end }}
})

// These are lightweight type tokens. 
const (
	_ {{ T "TypeID" }} = iota
{{ range $t := AllTypes }}{{ TypeID $t }};{{ end }}
)

// String is for debugging use only.
func (t {{ $TypeID }}) String() string {
	return {{ $Engine }}.Stringify(e.TypeID(t))
}
`
}
