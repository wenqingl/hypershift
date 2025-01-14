// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file defines the Const SSA value type.

import (
	"fmt"
	"go/constant"
	"go/types"
	"strconv"
	"strings"

	"honnef.co/go/tools/go/types/typeutil"
)

// NewConst returns a new constant of the specified value and type.
// val must be valid according to the specification of Const.Value.
//
func NewConst(val constant.Value, typ types.Type) *Const {
	return &Const{
		register: register{
			typ: typ,
		},
		Value: val,
	}
}

// intConst returns an 'int' constant that evaluates to i.
// (i is an int64 in case the host is narrower than the target.)
func intConst(i int64) *Const {
	return NewConst(constant.MakeInt64(i), tInt)
}

// nilConst returns a nil constant of the specified type, which may
// be any reference type, including interfaces.
//
func nilConst(typ types.Type) *Const {
	return NewConst(nil, typ)
}

// stringConst returns a 'string' constant that evaluates to s.
func stringConst(s string) *Const {
	return NewConst(constant.MakeString(s), tString)
}

// zeroConst returns a new "zero" constant of the specified type,
// which must not be an array or struct type: the zero values of
// aggregates are well-defined but cannot be represented by Const.
//
func zeroConst(t types.Type) Constant {
	switch t := t.(type) {
	case *types.Basic:
		switch {
		case t.Info()&types.IsBoolean != 0:
			return NewConst(constant.MakeBool(false), t)
		case t.Info()&types.IsNumeric != 0:
			return NewConst(constant.MakeInt64(0), t)
		case t.Info()&types.IsString != 0:
			return NewConst(constant.MakeString(""), t)
		case t.Kind() == types.UnsafePointer:
			fallthrough
		case t.Kind() == types.UntypedNil:
			return nilConst(t)
		case t.Kind() == types.Invalid:
			return nil
		default:
			panic(fmt.Sprint("zeroConst for unexpected type:", t))
		}
	case *types.Pointer, *types.Slice, *types.Interface, *types.Chan, *types.Map, *types.Signature, *typeutil.Iterator:
		return nilConst(t)
	case *types.Named:
		k := zeroConst(t.Underlying())
		switch k := k.(type) {
		case *Const:
			return NewConst(k.Value, t)
		case *AggregateConst:
			return &AggregateConst{
				register: register{
					typ: t,
				},
				Values: k.Values,
			}
		case *ArrayConst:
			return &ArrayConst{
				register: register{
					typ: t,
				},
			}
		default:
			panic(fmt.Sprintf("unexpected type %T", k))
		}
	case *types.Array:
		return &ArrayConst{
			register: register{
				typ: t,
			},
		}
	case *types.Struct:
		values := make([]Constant, t.NumFields())
		for i := 0; i < t.NumFields(); i++ {
			values[i] = zeroConst(t.Field(i).Type())
		}
		return &AggregateConst{
			register: register{typ: t},
			Values:   values,
		}
	case *types.Tuple:
		values := make([]Constant, t.Len())
		for i := 0; i < t.Len(); i++ {
			values[i] = zeroConst(t.At(i).Type())
		}
		return &AggregateConst{
			register: register{typ: t},
			Values:   values,
		}
	}

	panic(fmt.Sprint("zeroConst: unexpected ", t))
}

func (c *Const) RelString(from *types.Package) string {
	var p string
	if c.Value == nil {
		p = "nil"
	} else if c.Value.Kind() == constant.String {
		v := constant.StringVal(c.Value)
		const max = 20
		// TODO(adonovan): don't cut a rune in half.
		if len(v) > max {
			v = v[:max-3] + "..." // abbreviate
		}
		p = strconv.Quote(v)
	} else {
		p = c.Value.String()
	}
	return fmt.Sprintf("Const <%s> {%s}", relType(c.Type(), from), p)
}

func (c *Const) String() string {
	return c.RelString(c.Parent().pkg())
}

func (v *ArrayConst) RelString(pkg *types.Package) string {
	return fmt.Sprintf("ArrayConst <%s>", relType(v.Type(), pkg))
}

func (v *ArrayConst) String() string {
	return v.RelString(v.Parent().pkg())
}

func (v *AggregateConst) RelString(pkg *types.Package) string {
	values := make([]string, len(v.Values))
	for i, v := range v.Values {
		if v != nil {
			values[i] = v.RelString(pkg)
		} else {
			values[i] = "nil"
		}
	}
	return fmt.Sprintf("AggregateConst <%s> (%s)", relType(v.Type(), pkg), strings.Join(values, ", "))
}

func (v *AggregateConst) String() string {
	return v.RelString(v.Parent().pkg())
}

// IsNil returns true if this constant represents a typed or untyped nil value.
func (c *Const) IsNil() bool {
	return c.Value == nil
}

// Int64 returns the numeric value of this constant truncated to fit
// a signed 64-bit integer.
//
func (c *Const) Int64() int64 {
	switch x := constant.ToInt(c.Value); x.Kind() {
	case constant.Int:
		if i, ok := constant.Int64Val(x); ok {
			return i
		}
		return 0
	case constant.Float:
		f, _ := constant.Float64Val(x)
		return int64(f)
	}
	panic(fmt.Sprintf("unexpected constant value: %T", c.Value))
}

// Uint64 returns the numeric value of this constant truncated to fit
// an unsigned 64-bit integer.
//
func (c *Const) Uint64() uint64 {
	switch x := constant.ToInt(c.Value); x.Kind() {
	case constant.Int:
		if u, ok := constant.Uint64Val(x); ok {
			return u
		}
		return 0
	case constant.Float:
		f, _ := constant.Float64Val(x)
		return uint64(f)
	}
	panic(fmt.Sprintf("unexpected constant value: %T", c.Value))
}

// Float64 returns the numeric value of this constant truncated to fit
// a float64.
//
func (c *Const) Float64() float64 {
	f, _ := constant.Float64Val(c.Value)
	return f
}

// Complex128 returns the complex value of this constant truncated to
// fit a complex128.
//
func (c *Const) Complex128() complex128 {
	re, _ := constant.Float64Val(constant.Real(c.Value))
	im, _ := constant.Float64Val(constant.Imag(c.Value))
	return complex(re, im)
}

func (c *Const) equal(o Constant) bool {
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	oc, ok := o.(*Const)
	if !ok {
		return false
	}
	return c.typ == oc.typ && c.Value == oc.Value
}

func (c *AggregateConst) equal(o Constant) bool {
	oc, ok := o.(*AggregateConst)
	if !ok {
		return false
	}
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	return c.typ == oc.typ
}

func (c *ArrayConst) equal(o Constant) bool {
	oc, ok := o.(*ArrayConst)
	if !ok {
		return false
	}
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	return c.typ == oc.typ
}
