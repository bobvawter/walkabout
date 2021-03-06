// Code generated by github.com/cockroachdb/walkabout. DO NOT EDIT.
// source:

package demo

import (
	"fmt"
	"unsafe"

	e "github.com/cockroachdb/walkabout/engine"
)

// ------ API and public types ------

// CalcTypeID is a lightweight type token.
type CalcTypeID e.TypeID

// CalcAbstract allows users to treat a Calc as an abstract
// tree of nodes. All visitable struct types will have generated methods
// which implement this interface.
type CalcAbstract interface {
	// CalcAt returns the nth field of a struct or nth element of a
	// slice. If the child is a type which directly implements
	// CalcAbstract, it will be returned. If the child is of a pointer or
	// interface type, the value will be automatically dereferenced if it
	// is non-nil. If the child is a slice type, a CalcAbstract wrapper
	// around the slice will be returned.
	CalcAt(index int) CalcAbstract
	// CalcCount returns the number of visitable fields in a struct,
	// or the length of a slice.
	CalcCount() int
	// CalcTypeID returns a type token.
	CalcTypeID() CalcTypeID
}

var (
	_ CalcAbstract = &BinaryOp{}
	_ CalcAbstract = &Calculation{}
	_ CalcAbstract = &Func{}
	_ CalcAbstract = &Scalar{}
)

// CalcWalkerFn is used to implement a visitor pattern over
// types which implement Calc.
//
// Implementations of this function return a CalcDecision, which
// allows the function to control traversal. The zero value of
// CalcDecision means "continue". Other values can be obtained from the
// provided CalcContext to stop or to return an error.
//
// A CalcDecision can also specify a post-visit function to execute
// or can be used to replace the value being visited.
type CalcWalkerFn func(ctx CalcContext, x Calc) CalcDecision

// CalcContext is provided to CalcWalkerFn and acts as a factory
// for constructing CalcDecision instances.
type CalcContext struct {
	impl e.Context
}

// Actions will perform the given actions in place of visiting values
// that would normally be visited.  This allows callers to control
// specific field visitation order or to insert additional callbacks
// between visiting certain values.
func (c *CalcContext) Actions(actions ...CalcAction) CalcDecision {
	if actions == nil || len(actions) == 0 {
		return c.Skip()
	}

	ret := make([]e.Action, len(actions))
	for i, a := range actions {
		ret[i] = e.Action(a)
	}

	return CalcDecision(c.impl.Actions(ret))
}

// Continue returns the zero-value of CalcDecision. It exists only
// for cases where it improves the readability of code.
func (c *CalcContext) Continue() CalcDecision {
	return CalcDecision(c.impl.Continue())
}

// Error returns a CalcDecision which will cause the given error
// to be returned from the Walk() function. Post-visit functions
// will not be called.
func (c *CalcContext) Error(err error) CalcDecision {
	return CalcDecision(c.impl.Error(err))
}

// Halt will end a visitation early and return from the Walk() function.
// Any registered post-visit functions will be called.
func (c *CalcContext) Halt() CalcDecision {
	return CalcDecision(c.impl.Halt())
}

// Skip will not traverse the fields of the current object.
func (c *CalcContext) Skip() CalcDecision {
	return CalcDecision(c.impl.Skip())
}

// CalcDecision is used by CalcWalkerFn to control visitation.
// The CalcContext provided to a CalcWalkerFn acts as a factory
// for CalcDecision instances. In general, the factory methods
// choose a traversal strategy and additional methods on the
// CalcDecision can achieve a variety of side-effects.
type CalcDecision e.Decision

// Intercept registers a function to be called immediately before
// visiting each field or element of the current value.
func (d CalcDecision) Intercept(fn CalcWalkerFn) CalcDecision {
	return CalcDecision((e.Decision)(d).Intercept(fn))
}

// Post registers a post-visit function, which will be called after the
// fields of the current object. The function can make another decision
// about the current value.
func (d CalcDecision) Post(fn CalcWalkerFn) CalcDecision {
	return CalcDecision((e.Decision)(d).Post(fn))
}

// Replace allows the currently-visited value to be replaced. All
// parent nodes will be cloned.
func (d CalcDecision) Replace(x Calc) CalcDecision {
	return CalcDecision((e.Decision)(d).Replace(calcIdentify(x)))
}

// calcIdentify is a utility function to map a Calc into
// its generated type id and a pointer to the data.
func calcIdentify(x Calc) (typeId e.TypeID, data e.Ptr) {
	switch t := x.(type) {
	case *BinaryOp:
		typeId = e.TypeID(CalcTypeBinaryOp)
		data = e.Ptr(t)
	case *Calculation:
		typeId = e.TypeID(CalcTypeCalculation)
		data = e.Ptr(t)
	case *Func:
		typeId = e.TypeID(CalcTypeFunc)
		data = e.Ptr(t)
	case *Scalar:
		typeId = e.TypeID(CalcTypeScalar)
		data = e.Ptr(t)
	default:
		// The most probable reason for this is that the generated code
		// is out of date, or that an implementation of the Calc
		// interface from another package is being passed in.
		panic(fmt.Sprintf("unhandled value of type: %T", x))
	}
	return
}

// calcWrap is a utility function to reconstitute a Calc
// from an internal type token and a pointer to the value.
func calcWrap(typeId e.TypeID, x e.Ptr) Calc {
	switch CalcTypeID(typeId) {
	case CalcTypeBinaryOp:
		return (*BinaryOp)(x)
	case CalcTypeBinaryOpPtr:
		return *(**BinaryOp)(x)
	case CalcTypeCalculation:
		return (*Calculation)(x)
	case CalcTypeCalculationPtr:
		return *(**Calculation)(x)
	case CalcTypeFunc:
		return (*Func)(x)
	case CalcTypeFuncPtr:
		return *(**Func)(x)
	case CalcTypeScalar:
		return (*Scalar)(x)
	case CalcTypeScalarPtr:
		return *(**Scalar)(x)
	default:
		// This is likely a code-generation problem.
		panic(fmt.Sprintf("unhandled TypeID %d", typeId))
	}
}

// CalcAction is used by CalcContext.Actions() and allows users
// to have fine-grained control over traversal.
type CalcAction e.Action

// ActionVisit constructs a CalcAction that will visit the given value.
func (c *CalcContext) ActionVisit(x Calc) CalcAction {
	return CalcAction(c.impl.ActionVisitTypeID(calcIdentify(x)))
}

// ActionCall constructs a CalcAction that will invoke the given callback.
func (c *CalcContext) ActionCall(fn func() error) CalcAction {
	return CalcAction(c.impl.ActionCall(fn))
}

// ------ Type Enhancements ------

// calcAbstract is a type-safe facade around e.Abstract.
type calcAbstract struct {
	delegate *e.Abstract
}

var _ CalcAbstract = &calcAbstract{}

// CalcAt implements CalcAbstract.
func (a *calcAbstract) CalcAt(index int) (ret CalcAbstract) {
	impl := a.delegate.ChildAt(index)
	if impl == nil {
		return nil
	}
	switch CalcTypeID(impl.TypeID()) {
	case CalcTypeBinaryOp:
		ret = (*BinaryOp)(impl.Ptr())
	case CalcTypeBinaryOpPtr:
		ret = *(**BinaryOp)(impl.Ptr())
	case CalcTypeCalculation:
		ret = (*Calculation)(impl.Ptr())
	case CalcTypeCalculationPtr:
		ret = *(**Calculation)(impl.Ptr())
	case CalcTypeFunc:
		ret = (*Func)(impl.Ptr())
	case CalcTypeFuncPtr:
		ret = *(**Func)(impl.Ptr())
	case CalcTypeScalar:
		ret = (*Scalar)(impl.Ptr())
	case CalcTypeScalarPtr:
		ret = *(**Scalar)(impl.Ptr())
	default:
		ret = &calcAbstract{impl}
	}
	return
}

// CalcCount implements CalcAbstract.
func (a *calcAbstract) CalcCount() int {
	return a.delegate.NumChildren()
}

// CalcTypeID implements CalcAbstract.
func (a *calcAbstract) CalcTypeID() CalcTypeID {
	return CalcTypeID(a.delegate.TypeID())
}

// CalcAt implements CalcAbstract.
func (x *BinaryOp) CalcAt(index int) CalcAbstract {
	self := calcAbstract{calcEngine.Abstract(e.TypeID(CalcTypeBinaryOp), e.Ptr(x))}
	return self.CalcAt(index)
}

// CalcCount returns 2.
func (x *BinaryOp) CalcCount() int { return 2 }

// CalcTypeID returns CalcTypeBinaryOp.
func (*BinaryOp) CalcTypeID() CalcTypeID { return CalcTypeBinaryOp }

// WalkCalc visits the receiver with the provided callback.
func (x *BinaryOp) WalkCalc(fn CalcWalkerFn) (_ *BinaryOp, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = calcEngine.Execute(fn, e.TypeID(CalcTypeBinaryOp), e.Ptr(x), e.TypeID(CalcTypeBinaryOp))
	if err != nil {
		return nil, false, err
	}
	return (*BinaryOp)(y), changed, nil
}

// CalcAt implements CalcAbstract.
func (x *Calculation) CalcAt(index int) CalcAbstract {
	self := calcAbstract{calcEngine.Abstract(e.TypeID(CalcTypeCalculation), e.Ptr(x))}
	return self.CalcAt(index)
}

// CalcCount returns 1.
func (x *Calculation) CalcCount() int { return 1 }

// CalcTypeID returns CalcTypeCalculation.
func (*Calculation) CalcTypeID() CalcTypeID { return CalcTypeCalculation }

// WalkCalc visits the receiver with the provided callback.
func (x *Calculation) WalkCalc(fn CalcWalkerFn) (_ *Calculation, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = calcEngine.Execute(fn, e.TypeID(CalcTypeCalculation), e.Ptr(x), e.TypeID(CalcTypeCalculation))
	if err != nil {
		return nil, false, err
	}
	return (*Calculation)(y), changed, nil
}

// CalcAt implements CalcAbstract.
func (x *Func) CalcAt(index int) CalcAbstract {
	self := calcAbstract{calcEngine.Abstract(e.TypeID(CalcTypeFunc), e.Ptr(x))}
	return self.CalcAt(index)
}

// CalcCount returns 1.
func (x *Func) CalcCount() int { return 1 }

// CalcTypeID returns CalcTypeFunc.
func (*Func) CalcTypeID() CalcTypeID { return CalcTypeFunc }

// WalkCalc visits the receiver with the provided callback.
func (x *Func) WalkCalc(fn CalcWalkerFn) (_ *Func, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = calcEngine.Execute(fn, e.TypeID(CalcTypeFunc), e.Ptr(x), e.TypeID(CalcTypeFunc))
	if err != nil {
		return nil, false, err
	}
	return (*Func)(y), changed, nil
}

// CalcAt implements CalcAbstract.
func (x *Scalar) CalcAt(index int) CalcAbstract {
	self := calcAbstract{calcEngine.Abstract(e.TypeID(CalcTypeScalar), e.Ptr(x))}
	return self.CalcAt(index)
}

// CalcCount returns 0.
func (x *Scalar) CalcCount() int { return 0 }

// CalcTypeID returns CalcTypeScalar.
func (*Scalar) CalcTypeID() CalcTypeID { return CalcTypeScalar }

// WalkCalc visits the receiver with the provided callback.
func (x *Scalar) WalkCalc(fn CalcWalkerFn) (_ *Scalar, changed bool, err error) {
	var y e.Ptr
	_, y, changed, err = calcEngine.Execute(fn, e.TypeID(CalcTypeScalar), e.Ptr(x), e.TypeID(CalcTypeScalar))
	if err != nil {
		return nil, false, err
	}
	return (*Scalar)(y), changed, nil
}

// WalkCalc visits the receiver with the provided callback.
func WalkCalc(x Calc, fn CalcWalkerFn) (_ Calc, changed bool, err error) {
	id, ptr := calcIdentify(x)
	id, ptr, changed, err = calcEngine.Execute(fn, id, ptr, e.TypeID(CalcTypeCalc))
	if err != nil {
		return nil, false, err
	}
	if changed {
		return calcWrap(id, ptr), true, nil
	}
	return x, false, nil
}

// ------ Union Support -----
type Calc interface {
	CalcAbstract
	isCalcType()
}

var (
	_ Calc = &BinaryOp{}
	_ Calc = &Calculation{}
	_ Calc = &Func{}
	_ Calc = &Scalar{}
)

func (*BinaryOp) isCalcType()    {}
func (*Calculation) isCalcType() {}
func (*Func) isCalcType()        {}
func (*Scalar) isCalcType()      {} // ------ Type Mapping ------
var calcEngine = e.New(e.TypeMap{
	// ------ Structs ------
	CalcTypeBinaryOp: {
		Copy: func(dest, from e.Ptr) { *(*BinaryOp)(dest) = *(*BinaryOp)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(CalcWalkerFn)(CalcContext{impl}, (*BinaryOp)(x)))
		},
		Fields: []e.FieldInfo{
			{Name: "Left", Offset: unsafe.Offsetof(BinaryOp{}.Left), Target: e.TypeID(CalcTypeExpr)},
			{Name: "Right", Offset: unsafe.Offsetof(BinaryOp{}.Right), Target: e.TypeID(CalcTypeExpr)},
		},
		Name:      "BinaryOp",
		NewStruct: func() e.Ptr { return e.Ptr(&BinaryOp{}) },
		SizeOf:    unsafe.Sizeof(BinaryOp{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(CalcTypeBinaryOp),
	},
	CalcTypeCalculation: {
		Copy: func(dest, from e.Ptr) { *(*Calculation)(dest) = *(*Calculation)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(CalcWalkerFn)(CalcContext{impl}, (*Calculation)(x)))
		},
		Fields: []e.FieldInfo{
			{Name: "Expr", Offset: unsafe.Offsetof(Calculation{}.Expr), Target: e.TypeID(CalcTypeExpr)},
		},
		Name:      "Calculation",
		NewStruct: func() e.Ptr { return e.Ptr(&Calculation{}) },
		SizeOf:    unsafe.Sizeof(Calculation{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(CalcTypeCalculation),
	},
	CalcTypeFunc: {
		Copy: func(dest, from e.Ptr) { *(*Func)(dest) = *(*Func)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(CalcWalkerFn)(CalcContext{impl}, (*Func)(x)))
		},
		Fields: []e.FieldInfo{
			{Name: "Args", Offset: unsafe.Offsetof(Func{}.Args), Target: e.TypeID(CalcTypeExprSlice)},
		},
		Name:      "Func",
		NewStruct: func() e.Ptr { return e.Ptr(&Func{}) },
		SizeOf:    unsafe.Sizeof(Func{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(CalcTypeFunc),
	},
	CalcTypeScalar: {
		Copy: func(dest, from e.Ptr) { *(*Scalar)(dest) = *(*Scalar)(from) },
		Facade: func(impl e.Context, fn e.FacadeFn, x e.Ptr) e.Decision {
			return e.Decision(fn.(CalcWalkerFn)(CalcContext{impl}, (*Scalar)(x)))
		},
		Fields:    []e.FieldInfo{},
		Name:      "Scalar",
		NewStruct: func() e.Ptr { return e.Ptr(&Scalar{}) },
		SizeOf:    unsafe.Sizeof(Scalar{}),
		Kind:      e.KindStruct,
		TypeID:    e.TypeID(CalcTypeScalar),
	},

	// ------ Interfaces ------
	CalcTypeCalc: {
		Copy: func(dest, from e.Ptr) {
			*(*Calc)(dest) = *(*Calc)(from)
		},
		IntfType: func(x e.Ptr) e.TypeID {
			d := *(*Calc)(x)
			switch d.(type) {
			case *BinaryOp:
				return e.TypeID(CalcTypeBinaryOp)
			case *Calculation:
				return e.TypeID(CalcTypeCalculation)
			case *Func:
				return e.TypeID(CalcTypeFunc)
			case *Scalar:
				return e.TypeID(CalcTypeScalar)
			default:
				return 0
			}
		},
		IntfWrap: func(id e.TypeID, x e.Ptr) e.Ptr {
			var d Calc
			switch CalcTypeID(id) {
			case CalcTypeBinaryOp:
				d = (*BinaryOp)(x)
			case CalcTypeBinaryOpPtr:
				d = *(**BinaryOp)(x)
			case CalcTypeCalculation:
				d = (*Calculation)(x)
			case CalcTypeCalculationPtr:
				d = *(**Calculation)(x)
			case CalcTypeFunc:
				d = (*Func)(x)
			case CalcTypeFuncPtr:
				d = *(**Func)(x)
			case CalcTypeScalar:
				d = (*Scalar)(x)
			case CalcTypeScalarPtr:
				d = *(**Scalar)(x)
			default:
				return nil
			}
			return e.Ptr(&d)
		},
		Kind:   e.KindInterface,
		Name:   "Calc",
		SizeOf: unsafe.Sizeof(Calc(nil)),
		TypeID: e.TypeID(CalcTypeCalc),
	},
	CalcTypeExpr: {
		Copy: func(dest, from e.Ptr) {
			*(*Expr)(dest) = *(*Expr)(from)
		},
		IntfType: func(x e.Ptr) e.TypeID {
			d := *(*Expr)(x)
			switch d.(type) {
			case *BinaryOp:
				return e.TypeID(CalcTypeBinaryOp)
			case *Func:
				return e.TypeID(CalcTypeFunc)
			case *Scalar:
				return e.TypeID(CalcTypeScalar)
			default:
				return 0
			}
		},
		IntfWrap: func(id e.TypeID, x e.Ptr) e.Ptr {
			var d Expr
			switch CalcTypeID(id) {
			case CalcTypeBinaryOp:
				d = (*BinaryOp)(x)
			case CalcTypeBinaryOpPtr:
				d = *(**BinaryOp)(x)
			case CalcTypeFunc:
				d = (*Func)(x)
			case CalcTypeFuncPtr:
				d = *(**Func)(x)
			case CalcTypeScalar:
				d = (*Scalar)(x)
			case CalcTypeScalarPtr:
				d = *(**Scalar)(x)
			default:
				return nil
			}
			return e.Ptr(&d)
		},
		Kind:   e.KindInterface,
		Name:   "Expr",
		SizeOf: unsafe.Sizeof(Expr(nil)),
		TypeID: e.TypeID(CalcTypeExpr),
	},

	// ------ Pointers ------
	CalcTypeBinaryOpPtr: {
		Copy: func(dest, from e.Ptr) {
			*(**BinaryOp)(dest) = *(**BinaryOp)(from)
		},
		Elem:   e.TypeID(CalcTypeBinaryOp),
		SizeOf: unsafe.Sizeof((*BinaryOp)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(CalcTypeBinaryOpPtr),
	},
	CalcTypeCalculationPtr: {
		Copy: func(dest, from e.Ptr) {
			*(**Calculation)(dest) = *(**Calculation)(from)
		},
		Elem:   e.TypeID(CalcTypeCalculation),
		SizeOf: unsafe.Sizeof((*Calculation)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(CalcTypeCalculationPtr),
	},
	CalcTypeFuncPtr: {
		Copy: func(dest, from e.Ptr) {
			*(**Func)(dest) = *(**Func)(from)
		},
		Elem:   e.TypeID(CalcTypeFunc),
		SizeOf: unsafe.Sizeof((*Func)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(CalcTypeFuncPtr),
	},
	CalcTypeScalarPtr: {
		Copy: func(dest, from e.Ptr) {
			*(**Scalar)(dest) = *(**Scalar)(from)
		},
		Elem:   e.TypeID(CalcTypeScalar),
		SizeOf: unsafe.Sizeof((*Scalar)(nil)),
		Kind:   e.KindPointer,
		TypeID: e.TypeID(CalcTypeScalarPtr),
	},

	// ------ Slices ------
	CalcTypeExprSlice: {
		Copy: func(dest, from e.Ptr) {
			*(*[]Expr)(dest) = *(*[]Expr)(from)
		},
		Elem: e.TypeID(CalcTypeExpr),
		Kind: e.KindSlice,
		NewSlice: func(size int) e.Ptr {
			x := make([]Expr, size)
			return e.Ptr(&x)
		},
		SizeOf: unsafe.Sizeof(([]Expr)(nil)),
		TypeID: e.TypeID(CalcTypeExprSlice),
	},
})

// These are lightweight type tokens.
const (
	_ CalcTypeID = iota
	CalcTypeBinaryOp
	CalcTypeBinaryOpPtr
	CalcTypeCalc
	CalcTypeCalculation
	CalcTypeCalculationPtr
	CalcTypeExpr
	CalcTypeExprSlice
	CalcTypeFunc
	CalcTypeFuncPtr
	CalcTypeScalar
	CalcTypeScalarPtr
)

// String is for debugging use only.
func (t CalcTypeID) String() string {
	return calcEngine.Stringify(e.TypeID(t))
}
