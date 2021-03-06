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

// Package engine holds base implementation details for use by
// generated code. Users should not depend on any particular feature
// of this package.
package engine

import (
	"fmt"
	"reflect"
	"strings"
)

// Allows us to pre-allocate working space on the call stack.
const defaultStackDepth = 8

// See discussion on frame.Slots.
const fixedSlotCount = 16

// A frame represents the visitation of a single struct,
// interface, or slice.
type frame struct {
	// Count holds the number of slots to be visited.
	Count int
	// Idx is the current slot being visited.
	Idx       int
	Intercept FacadeFn
	// We keep a fixed-size array of slots per frame so that most
	// visitable objects won't need a heap allocation to store
	// the intermediate state.
	Slots [fixedSlotCount]Action
	// Large targets (such as slices) will use additional, heap-allocated
	// memory to store the intermediate state.
	Overflow []Action
}

// Active retrieves the active slot.
func (f *frame) Active() *Action {
	return f.Slot(f.Idx)
}

// Slot is used to access a storage slot within the frame.
func (f *frame) Slot(idx int) *Action {
	if idx < fixedSlotCount {
		return &f.Slots[idx]
	}
	return &f.Overflow[idx-fixedSlotCount]
}

// SetSlot is a helper function to configure a slot.
func (f *frame) SetSlot(e *Engine, idx int, action Action) *Action {
	ret := f.Slot(idx)
	*ret = action
	if ret.typeData == nil {
		ret.typeData = e.typeData(ret.valueType)
	}
	return ret
}

// Zero returns Slot(0).
func (f *frame) Zero() *Action {
	return &f.Slots[0]
}

// An Engine holds the necessary information to pass a visitor over
// a field.
type Engine struct {
	typeMap TypeMap
}

// New constructs an Engine.
func New(m TypeMap) *Engine {
	// Make a copy of the TypeMap and link all of the TypeDatas together.
	e := &Engine{typeMap: append(m[:0:0], m...)}
	for idx, td := range e.typeMap {
		if td.Elem != 0 {
			found := e.typeData(td.Elem)
			if found.TypeID == 0 {
				panic(fmt.Errorf("bad codegen: missing %d.Elem %d",
					td.TypeID, td.Elem))
			}
			e.typeMap[idx].elemData = found
		}

		for fIdx, field := range td.Fields {
			found := e.typeData(field.Target)
			if found.TypeID == 0 {
				panic(fmt.Errorf("bad codegen: missing %d.%s.Target %d",
					td.TypeID, field.Name, field.Target))
			}
			e.typeMap[idx].Fields[fIdx].targetData = found
		}
	}
	return e
}

// Abstract constructs an abstract accessor around a struct's field.
func (e *Engine) Abstract(typeID TypeID, x Ptr) *Abstract {
	if x == nil {
		return nil
	}
	return &Abstract{
		engine:   e,
		typeData: e.typeData(typeID),
		value:    x,
	}
}

// Execute drives the visitation process. This is an "unrolled
// recursive" function that maintains its own stack to avoid
// deeply-nested call stacks. We can also perform cycle-detection at
// fairly low cost. Any replacement of the top-level value must be
// assignable to the given TypeID.
func (e *Engine) Execute(
	fn FacadeFn, t TypeID, x Ptr, assignableTo TypeID,
) (retType TypeID, ret Ptr, changed bool, err error) {
	ctx := Context{}
	stack := newStack()

	// Bootstrap the stack.
	curFrame := stack.Enter(nil, 1)
	curSlot := curFrame.SetSlot(e, 0, ctx.ActionVisitReplace(e.typeData(t), x, e.typeData(assignableTo)))

	// Entering is a temporary pointer to the frame that we might be
	// entering into next, if the current value is a struct with fields, a
	// slice, etc.
	var entering *frame
	halting := false
	// This variable holds a pointer to a frame that we've just completed.
	// When we have a returning frame that's dirty, we'll want to unpack
	// its values into the current slot.
	var returning *frame

enter:
	if curSlot.call != nil {
		if err := curSlot.call(); err != nil {
			return 0, nil, false, err
		}
		goto unwind
	}

	// Linear search for cycle-breaking. Note that this does not guarantee
	// exactly-once behavior if there are multiple pointers to an object
	// within a visitable graph. pprof says this is much faster than using
	// a map structure, especially since we expect the stack to be fairly
	// shallow. We use both the type and pointer as a unique key in order
	// to distinguish a struct from the first field of the struct. go
	// disallows recursive type definitions, so it's impossible for the
	// first field of a struct to be exactly the struct type.
	for l := 0; l < stack.Depth()-1; l++ {
		onStack := stack.Peek(l).Active()
		if onStack.value == curSlot.value && onStack.typeData.TypeID == curSlot.typeData.TypeID {
			goto nextSlot
		}
	}

	// In this switch statement, we're going to set up the next frame. If
	// the current value doesn't need a new frame to be pushed, we'll jump
	// into the unwind block.
	switch curSlot.typeData.Kind {
	case KindPointer:
		// We dereference the pointer and push the resulting memory
		// location as a 1-slot frame.
		ptr := *(*Ptr)(curSlot.value)
		if ptr == nil {
			goto unwind
		}
		entering = stack.Enter(curFrame.Intercept, 1)
		entering.SetSlot(e, 0, ctx.ActionVisitReplace(curSlot.typeData.elemData, ptr, curSlot.typeData.elemData))

	case KindStruct:
		// Allow parent frames to intercept child values.
		if curFrame.Intercept != nil {
			d := curSlot.typeData.Facade(ctx, curFrame.Intercept, curSlot.value)
			if err := curSlot.apply(e, d); err != nil {
				return 0, nil, false, err
			}
			if d.halt {
				halting = true
			}
			// Allow interceptors to replace themselves.
			if d.intercept != nil {
				curFrame.Intercept = d.intercept
			}
		}

		// Structs are where we call out to user logic via a generated,
		// type-safe facade. The user code can trigger various flow-control
		// to happen.
		d := curSlot.typeData.Facade(ctx, fn, curSlot.value)
		// Incorporate replacements, bail on error, etc.
		if err := curSlot.apply(e, d); err != nil {
			return 0, nil, false, err
		}
		// If the user wants to stop, we'll set the flag and just let the
		// unwind loop run to completion.
		if d.halt {
			halting = true
		}
		// Slices and structs have very similar approaches, we create a new
		// frame, add slots for each field or slice element, and then jump
		// back to the top.
		fieldCount := len(curSlot.typeData.Fields)
		switch {
		case halting, d.skip:
			goto unwind

		case d.actions != nil:
			if len(d.actions) == 0 {
				goto unwind
			}
			entering = stack.Enter(d.intercept, len(d.actions))
			for i, a := range d.actions {
				entering.SetSlot(e, i, a)
			}

		default:
			if fieldCount == 0 {
				goto unwind
			}
			entering = stack.Enter(d.intercept, fieldCount)
			for i, f := range curSlot.typeData.Fields {
				fPtr := Ptr(uintptr(curSlot.value) + f.Offset)
				entering.SetSlot(e, i, ctx.ActionVisitReplace(f.targetData, fPtr, f.targetData))
			}
		}

	case KindSlice:
		// Slices have the same general flow as a struct; they're just
		// a sequence of visitable values.
		header := (*reflect.SliceHeader)(curSlot.value)
		if header.Len == 0 {
			goto unwind
		}
		entering = stack.Enter(curFrame.Intercept, header.Len)
		eltTd := curSlot.typeData.elemData
		for i, off := 0, uintptr(0); i < header.Len; i, off = i+1, off+eltTd.SizeOf {
			entering.SetSlot(e, i, ctx.ActionVisitReplace(eltTd, Ptr(header.Data+off), eltTd))
		}

	case KindInterface:
		// An interface is a type-tag and a pointer.
		ptr := (*[2]Ptr)(curSlot.value)[1]
		// We do need to map the type-tag to our TypeID.
		// Perhaps this could be accomplished with a map?
		elem := curSlot.typeData.IntfType(curSlot.value)
		// Need to check elem==0 in the case of a "typed nil" value.
		if elem == 0 || ptr == nil {
			goto unwind
		}
		entering = stack.Enter(curFrame.Intercept, 1)
		entering.SetSlot(e, 0, ctx.ActionVisitReplace(e.typeData(elem), ptr, curSlot.typeData))

	default:
		panic(fmt.Errorf("unexpected kind: %d", curSlot.typeData.Kind))
	}

	curFrame = entering
	curSlot = curFrame.Zero()

	// We've pushed a new frame onto the stack, so we'll restart.
	goto enter

unwind:
	// Execute any user-provided callback. This logic is pretty much
	// the same as above, although we don't respect all decision options.
	if curSlot.post != nil {
		d := curSlot.typeData.Facade(ctx, curSlot.post, curSlot.value)
		if err := curSlot.apply(e, d); err != nil {
			return 0, nil, false, err
		}
		if d.halt {
			halting = true
		}
	}

	// If the slot reports that it's dirty, we want to propagate
	// the changes upwards in the stack.
	if curSlot.dirty {
		if stack.Depth() > 1 {
			stack.Top(1).Active().dirty = true
		}

		// If we were given a replacement value, there's no need to
		// copy out any data.
		if !curSlot.replaced {
			// This switch statement is the inverse of the above. We'll fold the
			// returning frame into a replacement value for the current slot.
			switch curSlot.typeData.Kind {
			case KindStruct:
				// Allocate a replacement instance of the struct.
				next := curSlot.typeData.NewStruct()
				// Perform a shallow copy to catch non-visitable fields.
				curSlot.typeData.Copy(next, curSlot.value)

				// Copy the visitable fields into the new struct.
				for i, f := range curSlot.typeData.Fields {
					fPtr := Ptr(uintptr(next) + f.Offset)
					f.targetData.Copy(fPtr, returning.Slot(i).value)
				}
				curSlot.value = next

			case KindPointer:
				// Copy out the pointer to a local var so we don't stomp on it.
				next := returning.Zero().value
				curSlot.value = Ptr(&next)

			case KindSlice:
				// Create a new slice instance and populate the elements.
				next := curSlot.typeData.NewSlice(returning.Count)
				toHeader := (*reflect.SliceHeader)(next)
				elemTd := curSlot.typeData.elemData

				// Copy the elements across.
				for i := 0; i < returning.Count; i++ {
					toElem := Ptr(toHeader.Data + uintptr(i)*elemTd.SizeOf)
					elemTd.Copy(toElem, returning.Slot(i).value)
				}
				curSlot.value = next

			case KindInterface:
				// Swap out the iface pointer just like the pointer case above.
				next := returning.Zero()
				curSlot.value = curSlot.typeData.IntfWrap(next.typeData.TypeID, next.value)

			default:
				panic(fmt.Errorf("unimplemented: %d", curSlot.typeData.Kind))
			}
		}
	}

nextSlot:
	// We'll advance the current slot or unwind one level if we've
	// processed the last slot in the frame.
	curFrame.Idx++
	// If the user wants to stop early, we'll just keep running the
	// unwind loop until we hit the top frame.
	if curFrame.Idx == curFrame.Count || halting {
		// If we've finished the bootstrap frame, we're done.
		if stack.Depth() == 1 {
			// pprof says that this is measurably faster than repeatedly
			// dereferencing the pointer.
			z := *curFrame.Zero()
			return z.typeData.TypeID, z.value, z.dirty, nil
		}
		// Save off the current frame so we can copy the data out.
		returning = stack.Pop()
		curFrame = stack.Top(0)
		curSlot = curFrame.Active()
		// We'll jump back to the unwinding code to finish the slot of the
		// frame which is now on top.
		goto unwind
	} else {
		// We're just advancing to the next slot, so we jump back to the
		// top.
		curSlot = curFrame.Active()
		goto enter
	}
}

// Stringify returns a string representation of the given type that
// is suitable for debugging purposes.
func (e *Engine) Stringify(id TypeID) string {
	if id == 0 {
		return "<NIL>"
	}
	ret := strings.Builder{}
	td := e.typeData(id)
	for {
		switch td.Kind {
		case KindInterface, KindStruct:
			if ret.Len() == 0 {
				return td.Name
			}
			ret.WriteString(td.Name)
			return ret.String()
		case KindPointer:
			ret.WriteRune('*')
			td = td.elemData
		case KindSlice:
			ret.WriteString("[]")
			td = td.elemData
		default:
			panic(fmt.Errorf("unsupported: %d", td.Kind))
		}
	}
}

// typeData returns a pointer to the TypeData for the given type.
func (e *Engine) typeData(id TypeID) *TypeData {
	return &e.typeMap[id]
}
