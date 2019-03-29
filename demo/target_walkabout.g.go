package demo

import (
	"fmt"
	"unsafe"

	e "github.com/cockroachdb/walkabout/engine"
)

// ------ API and public types ------

// TargetTypeID is a lightweight type token.
type TargetTypeID e.TypeID

// TargetAbstract allows users to treat a Target as an abstract
// tree of nodes. All visitable struct types will have generated methods
// which implement this interface.
type TargetAbstract interface {
	// TargetAt returns the nth field of a struct or nth element of a
	// slice. If the child is a type which directly implements
	// TargetAbstract, it will be returned. If the child is of a pointer or
	// interface type, the value will be automatically dereferenced if it
	// is non-nil. If the child is a slice type, a TargetAbstract wrapper
	// around the slice will be returned.
	TargetAt(index int) TargetAbstract
	// TargetCount returns the number of visitable fields in a struct,
	// or the length of a slice.
	TargetCount() int
	// TargetTypeID returns a type token.
	TargetTypeID() TargetTypeID
}

var (
	_ TargetAbstract = &ByRefType{}
	_ TargetAbstract = &ByValType{}
	_ TargetAbstract = &ContainerType{}
	_ TargetAbstract = &ReachableType{}
	_ TargetAbstract = &UnionableType{}
)

// TargetWalkerFn is used to implement a visitor pattern over
// types which implement Target.
//
// Implementations of this function return a TargetDecision, which
// allows the function to control traversal. The zero value of
// TargetDecision means "continue". Other values can be obtained from the
// provided TargetContext to stop or to return an error.
//
// A TargetDecision can also specify a post-visit function to execute
// or can be used to replace the value being visited.
type TargetWalkerFn func(ctx TargetContext, x Target) TargetDecision

// TargetContext is provided to TargetWalkerFn and acts as a factory
// for constructing TargetDecision instances.
type TargetContext struct {
	impl e.Context
}

// Actions will perform the given actions in place of visiting values
// that would normally be visited.  This allows callers to control
// specific field visitation order or to insert additional callbacks
// between visiting certain values.
func (c *TargetContext) Actions(actions ...TargetAction) TargetDecision {
	if actions == nil || len(actions) == 0 {
		return c.Skip()
	}

	ret := make([]e.Action, len(actions))
	for i, a := range actions {
		ret[i] = e.Action(a)
	}

	return TargetDecision(c.impl.Actions(ret))
}

// Continue returns the zero-value of TargetDecision. It exists only
// for cases where it improves the readability of code.
func (c *TargetContext) Continue() TargetDecision {
	return TargetDecision(c.impl.Continue())
}

// Error returns a TargetDecision which will cause the given error
// to be returned from the Walk() function. Post-visit functions
// will not be called.
func (c *TargetContext) Error(err error) TargetDecision {
	return TargetDecision(c.impl.Error(err))
}

// Halt will end a visitation early and return from the Walk() function.
// Any registered post-visit functions will be called.
func (c *TargetContext) Halt() TargetDecision {
	return TargetDecision(c.impl.Halt())
}

// Skip will not traverse the fields of the current object.
func (c *TargetContext) Skip() TargetDecision {
	return TargetDecision(c.impl.Skip())
}

// TargetDecision is used by TargetWalkerFn to control visitation.
// The TargetContext provided to a TargetWalkerFn acts as a factory
// for TargetDecision instances. In general, the factory methods
// choose a traversal strategy and additional methods on the
// TargetDecision can achieve a variety of side-effects.
type TargetDecision e.Decision

// Intercept registers a function to be called immediately before
// visiting each field or element of the current value.
func (d TargetDecision) Intercept(fn TargetWalkerFn) TargetDecision {
	return TargetDecision((e.Decision)(d).Intercept(fn))
}

// Post registers a post-visit function, which will be called after the
// fields of the current object. The function can make another decision
// about the current value.
func (d TargetDecision) Post(fn TargetWalkerFn) TargetDecision {
	return TargetDecision((e.Decision)(d).Post(fn))
}

// Replace allows the currently-visited value to be replaced. All
// parent nodes will be cloned.
func (d TargetDecision) Replace(x Target) TargetDecision {
	return TargetDecision((e.Decision)(d).Replace(targetIdentify(x)))
}

// targetIdentify is a utility function to map a Target into
// its generated type id and a pointer to the data.
func targetIdentify(x Target) (typeId e.TypeID, data e.Ptr) {
	switch t := x.(type) {
	case *ByRefType:
		typeId = e.TypeID(TargetTypeByRefTypePtr)
		data = e.Ptr(t)
	case *ContainerType:
		typeId = e.TypeID(TargetTypeContainerTypePtr)
		data = e.Ptr(t)
	case ByValType:
		typeId = e.TypeID(TargetTypeByValType)
		data = e.Ptr(&t)
	case EmbedsTarget:
		typeId = e.TypeID(TargetTypeEmbedsTarget)
		data = e.Ptr(&t)
	case Target:
		typeId = e.TypeID(TargetTypeTarget)
		data = e.Ptr(&t)
	case Targets:
		typeId = e.TypeID(TargetTypeTargets)
		data = e.Ptr(&t)
	default:
		// The most probable reason for this is that the generated code
		// is out of date, or that an implementation of the Target
		// interface from another package is being passed in.
		panic(fmt.Sprintf("unhandled value of type: %T", x))
	}
	return
}

// targetWrap is a utility function to reconstitute a Target
// from an internal type token and a pointer to the value.
func targetWrap(typeId e.TypeID, x e.Ptr) Target {
	switch TargetTypeID(typeId) {

	default:
		// This is likely a code-generation problem.
		panic(fmt.Sprintf("unhandled TypeID %d", typeId))
	}
}

// TargetAction is used by TargetContext.Actions() and allows users
// to have fine-grained control over traversal.
type TargetAction e.Action

// ActionVisit constructs a TargetAction that will visit the given value.
func (c *TargetContext) ActionVisit(x Target) TargetAction {
	return TargetAction(c.impl.ActionVisitTypeID(targetIdentify(x)))
}

// ActionCall constructs a TargetAction that will invoke the given callback.
func (c *TargetContext) ActionCall(fn func() error) TargetAction {
	return TargetAction(c.impl.ActionCall(fn))
}

// ------ Type Enhancements ------

// targetAbstract is a type-safe facade around e.Abstract.
type targetAbstract struct {
	delegate *e.Abstract
}

var _ TargetAbstract = &targetAbstract{}

// TargetAt implements TargetAbstract.
func (a *targetAbstract) TargetAt(index int) (ret TargetAbstract) {
	impl := a.delegate.ChildAt(index)
	if impl == nil {
		return nil
	}
	switch TargetTypeID(impl.TypeID()) {
	case TargetTypeByRefType:
		ret = (*ByRefType)(impl.Ptr())
	case TargetTypeByRefTypePtr:
		ret = *(**ByRefType)(impl.Ptr())
	case TargetTypeByValType:
		ret = (*ByValType)(impl.Ptr())
	case TargetTypeByValTypePtr:
		ret = *(**ByValType)(impl.Ptr())
	case TargetTypeContainerType:
		ret = (*ContainerType)(impl.Ptr())
	case TargetTypeContainerTypePtr:
		ret = *(**ContainerType)(impl.Ptr())
	case TargetTypeEmbedsTarget:
		ret = (*EmbedsTarget)(impl.Ptr())
	case TargetTypeEmbedsTargetPtr:
		ret = *(**EmbedsTarget)(impl.Ptr())
	case TargetTypeReachableType:
		ret = (*ReachableType)(impl.Ptr())
	case TargetTypeReachableTypePtr:
		ret = *(**ReachableType)(impl.Ptr())
	case TargetTypeTarget:
		ret = (*Target)(impl.Ptr())
	case TargetTypeTargetPtr:
		ret = *(**Target)(impl.Ptr())
	case TargetTypeTargets:
		ret = (*Targets)(impl.Ptr())
	case TargetTypeTargetsPtr:
		ret = *(**Targets)(impl.Ptr())
	case TargetTypeUnionableType:
		ret = (*UnionableType)(impl.Ptr())
	case TargetTypeUnionableTypePtr:
		ret = *(**UnionableType)(impl.Ptr())
	default:
		ret = &targetAbstract{impl}
	}
	return
}

// TargetCount implements TargetAbstract.
func (a *targetAbstract) TargetCount() int {
	return a.delegate.NumChildren()
}

// TargetTypeID implements TargetAbstract.
func (a *targetAbstract) TargetTypeID() TargetTypeID {
	return TargetTypeID(a.delegate.TypeID())
}

// TargetAt implements TargetAbstract.
func (x *ByRefType) TargetAt(index int) TargetAbstract {
	self := targetAbstract{targetEngine.Abstract(e.TypeID(TargetTypeByRefType), e.Ptr(x))}
	return self.TargetAt(index)
}

// TargetCount returns 1.
func (x *ByRefType) TargetCount() int { return 1 }

// TargetTypeID returns TargetTypeByRefType.
func (*ByRefType) TargetTypeID() TargetTypeID { return TargetTypeByRefType }

// WalkTarget visits the receiver with the provided callback.
func (x *ByRefType) WalkTarget(fn TargetWalkerFn) (_ *ByRefType, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = targetEngine.Execute(fn, e.TypeID(TargetTypeByRefType), e.Ptr(x), e.TypeID(TargetTypeByRefType))
	if err != nil {
		return nil, false, err
	}
	return (*ByRefType)(y), changed, nil
}

// TargetAt implements TargetAbstract.
func (x *ByValType) TargetAt(index int) TargetAbstract {
	self := targetAbstract{targetEngine.Abstract(e.TypeID(TargetTypeByValType), e.Ptr(x))}
	return self.TargetAt(index)
}

// TargetCount returns 1.
func (x *ByValType) TargetCount() int { return 1 }

// TargetTypeID returns TargetTypeByValType.
func (*ByValType) TargetTypeID() TargetTypeID { return TargetTypeByValType }

// WalkTarget visits the receiver with the provided callback.
func (x *ByValType) WalkTarget(fn TargetWalkerFn) (_ *ByValType, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = targetEngine.Execute(fn, e.TypeID(TargetTypeByValType), e.Ptr(x), e.TypeID(TargetTypeByValType))
	if err != nil {
		return nil, false, err
	}
	return (*ByValType)(y), changed, nil
}

// TargetAt implements TargetAbstract.
func (x *ContainerType) TargetAt(index int) TargetAbstract {
	self := targetAbstract{targetEngine.Abstract(e.TypeID(TargetTypeContainerType), e.Ptr(x))}
	return self.TargetAt(index)
}

// TargetCount returns 18.
func (x *ContainerType) TargetCount() int { return 18 }

// TargetTypeID returns TargetTypeContainerType.
func (*ContainerType) TargetTypeID() TargetTypeID { return TargetTypeContainerType }

// WalkTarget visits the receiver with the provided callback.
func (x *ContainerType) WalkTarget(fn TargetWalkerFn) (_ *ContainerType, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = targetEngine.Execute(fn, e.TypeID(TargetTypeContainerType), e.Ptr(x), e.TypeID(TargetTypeContainerType))
	if err != nil {
		return nil, false, err
	}
	return (*ContainerType)(y), changed, nil
}

// TargetAt implements TargetAbstract.
func (x *ReachableType) TargetAt(index int) TargetAbstract {
	self := targetAbstract{targetEngine.Abstract(e.TypeID(TargetTypeReachableType), e.Ptr(x))}
	return self.TargetAt(index)
}

// TargetCount returns 0.
func (x *ReachableType) TargetCount() int { return 0 }

// TargetTypeID returns TargetTypeReachableType.
func (*ReachableType) TargetTypeID() TargetTypeID { return TargetTypeReachableType }

// WalkTarget visits the receiver with the provided callback.
func (x *ReachableType) WalkTarget(fn TargetWalkerFn) (_ *ReachableType, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = targetEngine.Execute(fn, e.TypeID(TargetTypeReachableType), e.Ptr(x), e.TypeID(TargetTypeReachableType))
	if err != nil {
		return nil, false, err
	}
	return (*ReachableType)(y), changed, nil
}

// TargetAt implements TargetAbstract.
func (x *UnionableType) TargetAt(index int) TargetAbstract {
	self := targetAbstract{targetEngine.Abstract(e.TypeID(TargetTypeUnionableType), e.Ptr(x))}
	return self.TargetAt(index)
}

// TargetCount returns 0.
func (x *UnionableType) TargetCount() int { return 0 }

// TargetTypeID returns TargetTypeUnionableType.
func (*UnionableType) TargetTypeID() TargetTypeID { return TargetTypeUnionableType }

// WalkTarget visits the receiver with the provided callback.
func (x *UnionableType) WalkTarget(fn TargetWalkerFn) (_ *UnionableType, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = targetEngine.Execute(fn, e.TypeID(TargetTypeUnionableType), e.Ptr(x), e.TypeID(TargetTypeUnionableType))
	if err != nil {
		return nil, false, err
	}
	return (*UnionableType)(y), changed, nil
}

// WalkTarget visits the receiver with the provided callback.
func WalkTarget(x Target, fn TargetWalkerFn) (_ Target, changed bool, err error) {
	id, ptr := targetIdentify(x)
	id, ptr, changed, err = targetEngine.Execute(fn, id, ptr, e.TypeID(TargetTypeTarget))
	if err != nil {
		return nil, false, err
	}
	if changed {
		return targetWrap(id, ptr), true, nil
	}
	return x, false, nil
}

// ------ Type Mapping ------
var targetEngine = e.New(e.TypeMap{
	// ------ Structs ------
	TargetTypeByRefType: {
		Copy: func(dest, from e.Ptr) { *(*ByRefType)(dest) = *(*ByRefType)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(TargetWalkerFn)(TargetContext{impl}, (*ByRefType)(x)))
		},
		Fields:    []e.FieldInfo{},
		Name:      "ByRefType",
		NewStruct: func() e.Ptr { return e.Ptr(&ByRefType{}) },
		SizeOf:    unsafe.Sizeof(ByRefType{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(TargetTypeByRefType),
	},
	TargetTypeByValType: {
		Copy: func(dest, from e.Ptr) { *(*ByValType)(dest) = *(*ByValType)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(TargetWalkerFn)(TargetContext{impl}, (*ByValType)(x)))
		},
		Fields:    []e.FieldInfo{},
		Name:      "ByValType",
		NewStruct: func() e.Ptr { return e.Ptr(&ByValType{}) },
		SizeOf:    unsafe.Sizeof(ByValType{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(TargetTypeByValType),
	},
	TargetTypeContainerType: {
		Copy: func(dest, from e.Ptr) { *(*ContainerType)(dest) = *(*ContainerType)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(TargetWalkerFn)(TargetContext{impl}, (*ContainerType)(x)))
		},
		Fields: []e.FieldInfo{

			{Name: "Container", Offset: unsafe.Offsetof(ContainerType{}.Container), Target: e.TypeID(TargetTypeContainerTypePtr)},
			{Name: "AnotherTarget", Offset: unsafe.Offsetof(ContainerType{}.AnotherTarget), Target: e.TypeID(TargetTypeTarget)},
			{Name: "AnotherTargetPtr", Offset: unsafe.Offsetof(ContainerType{}.AnotherTargetPtr), Target: e.TypeID(TargetTypeTargetPtr)},
			{Name: "TargetSlice", Offset: unsafe.Offsetof(ContainerType{}.TargetSlice), Target: e.TypeID(TargetTypeTargetSlice)},
			{Name: "InterfacePtrSlice", Offset: unsafe.Offsetof(ContainerType{}.InterfacePtrSlice), Target: e.TypeID(TargetTypeTargetPtrSlice)},
			{Name: "NamedTargets", Offset: unsafe.Offsetof(ContainerType{}.NamedTargets), Target: e.TypeID(TargetTypeTargets)},
		},
		Name:      "ContainerType",
		NewStruct: func() e.Ptr { return e.Ptr(&ContainerType{}) },
		SizeOf:    unsafe.Sizeof(ContainerType{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(TargetTypeContainerType),
	},
	TargetTypeReachableType: {
		Copy: func(dest, from e.Ptr) { *(*ReachableType)(dest) = *(*ReachableType)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(TargetWalkerFn)(TargetContext{impl}, (*ReachableType)(x)))
		},
		Fields:    []e.FieldInfo{},
		Name:      "ReachableType",
		NewStruct: func() e.Ptr { return e.Ptr(&ReachableType{}) },
		SizeOf:    unsafe.Sizeof(ReachableType{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(TargetTypeReachableType),
	},
	TargetTypeUnionableType: {
		Copy: func(dest, from e.Ptr) { *(*UnionableType)(dest) = *(*UnionableType)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(TargetWalkerFn)(TargetContext{impl}, (*UnionableType)(x)))
		},
		Fields:    []e.FieldInfo{},
		Name:      "UnionableType",
		NewStruct: func() e.Ptr { return e.Ptr(&UnionableType{}) },
		SizeOf:    unsafe.Sizeof(UnionableType{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(TargetTypeUnionableType),
	},

	// ------ Interfaces ------
	TargetTypeEmbedsTarget: {
		Copy: func(dest, from e.Ptr) {
			*(*EmbedsTarget)(dest) = *(*EmbedsTarget)(from)
		},
		IntfType: func(x e.Ptr) e.TypeID {
			d := *(*EmbedsTarget)(x)
			switch d.(type) {
			case ByValType:
				return e.TypeID(TargetTypeByValType)
			case EmbedsTarget:
				return e.TypeID(TargetTypeEmbedsTarget)
			default:
				return 0
			}
		},
		IntfWrap: func(id e.TypeID, x e.Ptr) e.Ptr {
			var d EmbedsTarget
			switch TargetTypeID(id) {

			default:
				return nil
			}
			return e.Ptr(&d)
		},
		Kind:   e.KindInterface,
		Name:   "EmbedsTarget",
		SizeOf: unsafe.Sizeof(EmbedsTarget(nil)),
		TypeID: e.TypeID(TargetTypeEmbedsTarget),
	},
	TargetTypeTarget: {
		Copy: func(dest, from e.Ptr) {
			*(*Target)(dest) = *(*Target)(from)
		},
		IntfType: func(x e.Ptr) e.TypeID {
			d := *(*Target)(x)
			switch d.(type) {
			case *ByRefType:
				return e.TypeID(TargetTypeByRefTypePtr)
			case *ContainerType:
				return e.TypeID(TargetTypeContainerTypePtr)
			case ByValType:
				return e.TypeID(TargetTypeByValType)
			case EmbedsTarget:
				return e.TypeID(TargetTypeEmbedsTarget)
			case Target:
				return e.TypeID(TargetTypeTarget)
			case Targets:
				return e.TypeID(TargetTypeTargets)
			default:
				return 0
			}
		},
		IntfWrap: func(id e.TypeID, x e.Ptr) e.Ptr {
			var d Target
			switch TargetTypeID(id) {
			case TargetTypeByRefType:
				d = (*ByRefType)(x)
			case TargetTypeByRefTypePtr:
				d = *(**ByRefType)(x)
			case TargetTypeContainerType:
				d = (*ContainerType)(x)
			case TargetTypeContainerTypePtr:
				d = *(**ContainerType)(x)
			default:
				return nil
			}
			return e.Ptr(&d)
		},
		Kind:   e.KindInterface,
		Name:   "Target",
		SizeOf: unsafe.Sizeof(Target(nil)),
		TypeID: e.TypeID(TargetTypeTarget),
	},

	// ------ Pointers ------
	TargetTypeByRefTypePtr: {
		Copy: func(dest, from e.Ptr) {
			*(**ByRefType)(dest) = *(**ByRefType)(from)
		},
		Elem:   e.TypeID(TargetTypeByRefType),
		SizeOf: unsafe.Sizeof((*ByRefType)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(TargetTypeByRefTypePtr),
	},
	TargetTypeByValTypePtr: {
		Copy: func(dest, from e.Ptr) {
			*(**ByValType)(dest) = *(**ByValType)(from)
		},
		Elem:   e.TypeID(TargetTypeByValType),
		SizeOf: unsafe.Sizeof((*ByValType)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(TargetTypeByValTypePtr),
	},
	TargetTypeContainerTypePtr: {
		Copy: func(dest, from e.Ptr) {
			*(**ContainerType)(dest) = *(**ContainerType)(from)
		},
		Elem:   e.TypeID(TargetTypeContainerType),
		SizeOf: unsafe.Sizeof((*ContainerType)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(TargetTypeContainerTypePtr),
	},
	TargetTypeEmbedsTargetPtr: {
		Copy: func(dest, from e.Ptr) {
			*(**EmbedsTarget)(dest) = *(**EmbedsTarget)(from)
		},
		Elem:   e.TypeID(TargetTypeEmbedsTarget),
		SizeOf: unsafe.Sizeof((*EmbedsTarget)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(TargetTypeEmbedsTargetPtr),
	},
	TargetTypeTargetPtr: {
		Copy: func(dest, from e.Ptr) {
			*(**Target)(dest) = *(**Target)(from)
		},
		Elem:   e.TypeID(TargetTypeTarget),
		SizeOf: unsafe.Sizeof((*Target)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(TargetTypeTargetPtr),
	},
	TargetTypeUnionableTypePtr: {
		Copy: func(dest, from e.Ptr) {
			*(**UnionableType)(dest) = *(**UnionableType)(from)
		},
		Elem:   e.TypeID(TargetTypeUnionableType),
		SizeOf: unsafe.Sizeof((*UnionableType)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(TargetTypeUnionableTypePtr),
	},

	// ------ Slices ------
	TargetTypeTargets: {
		Copy: func(dest, from e.Ptr) {
			*(*Targets)(dest) = *(*Targets)(from)
		},
		Elem: e.TypeID(TargetTypeTarget),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make(Targets, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof((Targets)(nil)),
		TypeID: e.TypeID(TargetTypeTargets),
	},
	TargetTypeByRefTypePtrSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]*ByRefType)(dest) = *(*[]*ByRefType)(from)
		},
		Elem: e.TypeID(TargetTypeByRefTypePtr),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]*ByRefType, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]*ByRefType)(nil)),
		TypeID: e.TypeID(TargetTypeByRefTypePtrSlice),
	},
	TargetTypeByValTypePtrSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]*ByValType)(dest) = *(*[]*ByValType)(from)
		},
		Elem: e.TypeID(TargetTypeByValTypePtr),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]*ByValType, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]*ByValType)(nil)),
		TypeID: e.TypeID(TargetTypeByValTypePtrSlice),
	},
	TargetTypeTargetPtrSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]*Target)(dest) = *(*[]*Target)(from)
		},
		Elem: e.TypeID(TargetTypeTargetPtr),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]*Target, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]*Target)(nil)),
		TypeID: e.TypeID(TargetTypeTargetPtrSlice),
	},
	TargetTypeByRefTypeSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]ByRefType)(dest) = *(*[]ByRefType)(from)
		},
		Elem: e.TypeID(TargetTypeByRefType),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]ByRefType, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]ByRefType)(nil)),
		TypeID: e.TypeID(TargetTypeByRefTypeSlice),
	},
	TargetTypeByValTypeSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]ByValType)(dest) = *(*[]ByValType)(from)
		},
		Elem: e.TypeID(TargetTypeByValType),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]ByValType, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]ByValType)(nil)),
		TypeID: e.TypeID(TargetTypeByValTypeSlice),
	},
	TargetTypeTargetSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]Target)(dest) = *(*[]Target)(from)
		},
		Elem: e.TypeID(TargetTypeTarget),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]Target, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]Target)(nil)),
		TypeID: e.TypeID(TargetTypeTargetSlice),
	},

	// ------ Opaque ------

})

// These are lightweight type tokens.
const (
	_ TargetTypeID = iota
	TargetTypeByRefTypePtr
	TargetTypeByValTypePtr
	TargetTypeContainerTypePtr
	TargetTypeEmbedsTargetPtr
	TargetTypeTargetPtr
	TargetTypeUnionableTypePtr
	TargetTypeByRefType
	TargetTypeByValType
	TargetTypeContainerType
	TargetTypeEmbedsTarget
	TargetTypeReachableType
	TargetTypeTarget
	TargetTypeTargets
	TargetTypeUnionableType
	TargetTypeByRefTypePtrSlice
	TargetTypeByValTypePtrSlice
	TargetTypeTargetPtrSlice
	TargetTypeByRefTypeSlice
	TargetTypeByValTypeSlice
	TargetTypeTargetSlice
)

// String is for debugging use only.
func (t TargetTypeID) String() string {
	return targetEngine.Stringify(e.TypeID(t))
}
