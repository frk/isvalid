package analysis

import (
	"go/types"
	"strconv"
	"strings"

	"github.com/frk/tagutil"
)

type (
	// ValidatorStruct represents the result of the analysis of a target struct type.
	ValidatorStruct struct {
		// Name of the validator struct type.
		TypeName string
		// The primary fields of the validator struct.
		Fields []*StructField
		// Info on the isvalid.ErrorConstructor or isvalid.ErrorAggregator
		// field of the validator struct type, or nil.
		ErrorHandler *ErrorHandlerField
		// Info on the validator type's field named "context" (case insensitive), or nil.
		ContextOption *ContextOptionField
		// Info on the validator type's "beforevalidate" (case insensitive) method.
		BeforeValidate *MethodInfo
		// Info on the validator type's "aftervalidate" (case insensitive) method.
		AfterValidate *MethodInfo
	}

	// StructField describes a single struct field in a ValidatorStruct or
	// in any of a ValidatorStruct's members that themselves are structs.
	StructField struct {
		// Name of the field.
		Name string
		// The key of the StructField (used for errors, field args, etc.),
		// the value of this is determined by the "field key" settings, if not
		// specified by the user it will default to the value of the field's name.
		//
		// The Key of each field and subfield of a single ValidatorStruct will be unique.
		Key string
		// The field's type.
		Type Type
		// The field's parsed tag.
		Tag tagutil.Tag
		// Indicates whether or not the field is embedded.
		IsEmbedded bool
		// Indicates whether or not the field is exported.
		IsExported bool
		// The list of validation rules, as parsed from the struct's tag,
		// that need to be applied to the field.
		Rules []*RuleTag
	}

	// StructFieldSelector is a list of fields that represents a chain of
	// selectors where, the 0th field is the "root" field and the len-1
	// field is the "leaf" field.
	StructFieldSelector []*StructField

	// Type is the representation of a Go type.
	Type struct {
		// The name of a named type or empty string for unnamed types
		Name string
		// The kind of the go type.
		Kind TypeKind
		// The package import path.
		PkgPath string
		// The package's name.
		PkgName string
		// The local package name (including ".").
		PkgLocal string
		// Indicates whether or not the package is imported.
		IsImported bool
		// Indicates whether or not the field is exported.
		IsExported bool
		// If the base type's an array type, this field will hold the array's length.
		ArrayLen int64
		// Indicates whether or not the type is an empty interface type.
		IsEmptyInterface bool
		// Indicates whether or not the type is the "byte" alias type.
		IsByte bool
		// Indicates whether or not the type is the "rune" alias type.
		IsRune bool
		// If kind is map, key will hold the info on the map's key type.
		Key *Type
		// If kind is map, elem will hold the info on the map's value type.
		// If kind is ptr, elem will hold the info on pointed-to type.
		// If kind is slice/array, elem will hold the info on slice/array element type.
		Elem *Type
		// If kind is struct, Fields will hold the list of the struct's fields.
		Fields []*StructField
	}

	// ErrorHandlerField is the result of analyzing a validator struct's field whose
	// type implements the isvalid.ErrorConstructor or isvalid.ErrorAggregator interface.
	ErrorHandlerField struct {
		// Name of the field (case preserved).
		Name string
		// Indicates whether or not the field's type implements
		// the isvalid.ErrorAggregator interface.
		IsAggregator bool
	}

	// ContextOptionField is the result of analyzing a validator struct's
	// field whose name is equal to "context" (case insensitive).
	ContextOptionField struct {
		// Name of the field (case preserved).
		Name string
	}

	// MethodInfo represents the result of analysing a type's method.
	MethodInfo struct {
		// The name of the method (case preserved).
		Name string
	}

	// RuleTag holds the information parsed from a "rule" tag (`is:"{rule}"`).
	RuleTag struct {
		// Name of the rule.
		Name string
		// The args of the rule.
		Args []*RuleArg
		// The context in which the rule should be applied.
		Context string
		// XXX currently not implemented, may never be...
		SetKey string
	}

	// RuleArg represents a rule argument as parsed from a "rule" tag (`is:"{rule:arg}"`).
	RuleArg struct {
		// The type of the arg value.
		Type ArgType
		// The arg value, may be empty string.
		Value string
	}

	// RuleSpec implementations specify the validity of a field-rule
	// combo, as well as what code should be generated from a rule.
	RuleSpec interface {
		ruleSpec()

		IsCustom() bool
	}

	// RuleBasic represents a rule that uses the basic comparison
	// operators for carrying out its validation.
	RuleBasic struct {
		// Used for type-checking a rule's associated RuleTag and StructField.
		// For RuleBasic this field is expected to be non-nil.
		check func(a *analysis, f *StructField, r *RuleTag) error
	}

	// RuleFunc represents a rule that uses functions for carrying out its
	// validation.
	RuleFunc struct {
		// The name of the function.
		FuncName string
		// The function's package import path.
		PkgPath string
		// The types of the arguments to the function. Will always be of
		// length at least 1 where the 0th argument represents the field
		// to be validated.
		ArgTypes []Type
		// Indicates whether or not the function's signature is variadic.
		IsVariadic bool
		// Optional, indicates the boolean operator to be used between
		// multiple calls of the function represented by RuleFunc.
		// NOTE This can only be used with functions that take exactly
		// two arguments and it should not be variadic.
		BoolConn RuleFuncBoolConn
		// Indicates that the generated code should use raw strings
		// for any string arguments passed to the function.
		UseRawStrings bool

		// Optional, used for additional function-specific type
		// checking of the associated RuleTag and StructField.
		check func(a *analysis, f *StructField, r *RuleTag) error
		// Indicates that this RuleFunc is a custom one.
		iscustom bool
	}
)

func (RuleBasic) ruleSpec()      {}
func (RuleBasic) IsCustom() bool { return false }

func (RuleFunc) ruleSpec()        {}
func (f RuleFunc) IsCustom() bool { return f.iscustom }

func (t Type) String() string {
	if len(t.Name) > 0 {
		if t.IsImported {
			return t.PkgName + "." + t.Name
		}
		return t.Name
	}

	if t.IsByte {
		return "byte"
	} else if t.IsRune {
		return "rune"
	} else if t.Kind.IsBasic() {
		return typeKinds[t.Kind]
	}

	switch t.Kind {
	case TypeKindArray:
		return "[" + strconv.FormatInt(t.ArrayLen, 10) + "]" + t.Elem.String()
	case TypeKindInterface:
		if !t.IsEmptyInterface {
			return "interface{ ... }"
		}
		return "interface{}"
	case TypeKindMap:
		return "map[" + t.Key.String() + "]" + t.Elem.String()
	case TypeKindPtr:
		return "*" + t.Elem.String()
	case TypeKindSlice:
		return "[]" + t.Elem.String()
	case TypeKindStruct:
		if len(t.Fields) > 0 {
			return "struct{ ... }"
		}
		return "struct{}"
	case TypeKindChan:
		return "<chan>"
	case TypeKindFunc:
		return "<func>"
	}
	return "<unknown>"
}

func (t Type) PtrBase() Type {
	for t.Kind == TypeKindPtr {
		t = *t.Elem
	}
	return t
}

// Reports whether the types represented by t and u are equal. Note that this
// does not handle unnamed struct, interface (non-empty), func, and channel types.
func (t Type) Equals(u Type) bool {
	if t.Kind != u.Kind {
		return false
	}

	if len(t.Name) > 0 || len(u.Name) > 0 {
		return t.Name == u.Name && t.PkgPath == u.PkgPath
	}
	if t.Kind.IsBasic() {
		return t.Kind == u.Kind
	}

	switch t.Kind {
	case TypeKindArray:
		return t.ArrayLen == u.ArrayLen && t.Elem.Equals(*u.Elem)
	case TypeKindMap:
		return t.Key.Equals(*u.Key) && t.Elem.Equals(*u.Elem)
	case TypeKindSlice, TypeKindPtr:
		return t.Elem.Equals(*u.Elem)
	case TypeKindInterface:
		return t.IsEmptyInterface && u.IsEmptyInterface
	}
	return false
}

// Reports whether or not a value of type t needs to be converted before
// it can be assigned to a variable of type u.
func (t Type) NeedsConversion(u Type) bool {
	if u.Equals(t) {
		return false
	}
	if u.IsEmptyInterface {
		return false
	}
	return true
}

func (f *StructField) RulesCopy() []*RuleTag {
	if f.Rules == nil {
		return nil
	}

	rules := make([]*RuleTag, len(f.Rules))
	copy(rules, f.Rules)
	return rules
}

func (f *StructField) SubFields() []*StructField {
	typ := f.Type
	for typ.Kind == TypeKindPtr {
		typ = *typ.Elem // deref pointer
	}
	if typ.Kind == TypeKindStruct {
		return typ.Fields
	}
	return nil
}

func (f *StructField) HasRuleRequired() bool {
	for _, r := range f.Rules {
		if r.Name == "required" {
			return true
		}
	}
	return false
}

func (f *StructField) HasRuleNotnil() bool {
	for _, r := range f.Rules {
		if r.Name == "notnil" {
			return true
		}
	}
	return false
}

func (s StructFieldSelector) Last() *StructField {
	return s[len(s)-1]
}

func (a *RuleArg) IsUInt() bool {
	return a.Type == ArgTypeInt && a.Value[0] != '-'
}

func (f *RuleFunc) PkgName() string {
	if len(f.PkgPath) > 0 {
		if i := strings.LastIndexByte(f.PkgPath, '/'); i > -1 {
			return f.PkgPath[i+1:]
		}
		return f.PkgPath
	}
	return ""
}

// TypesForArgs returns an adjusted version of the RuleFunc's ArgTypes slice.
// The returned Type slice will match in length the given slice of RuleArgs.
func (f *RuleFunc) TypesForArgs(args []*RuleArg) (types []Type) {
	types = append(types, f.ArgTypes[1:]...)
	if f.IsVariadic {
		last := f.ArgTypes[len(f.ArgTypes)-1].Elem
		if len(types) > 0 {
			types[len(types)-1] = *last
		} else {
			types = []Type{*last}
		}

		diff := len(args) - len(types)
		for i := 0; i < diff; i++ {
			types = append(types, *last)
		}
		return types
	}

	last := f.ArgTypes[len(f.ArgTypes)-1]
	diff := len(args) - len(types)
	for i := 0; i < diff; i++ {
		types = append(types, last)
	}
	return types
}

// ArgType indicates the type of a rule arg value.
type ArgType uint

const (
	ArgTypeUnknown ArgType = iota
	ArgTypeBool
	ArgTypeInt
	ArgTypeFloat
	ArgTypeString
	ArgTypeField
)

// RuleFuncBoolConn indicates the boolean connective to be used
// between multiple alls of a single RuleFunc.
type RuleFuncBoolConn uint

const (
	RuleFuncBoolNone RuleFuncBoolConn = iota
	RuleFuncBoolNot
	RuleFuncBoolAnd
	RuleFuncBoolOr
)

// TypeKind indicates the specific kind of a Go type.
type TypeKind uint

const (
	// basic
	TypeKindInvalid TypeKind = iota

	_basic_kind_start
	TypeKindBool
	_numeric_kind_start // int/uint/float
	_integer_kind_start // int
	TypeKindInt
	TypeKindInt8
	TypeKindInt16
	TypeKindInt32
	TypeKindInt64
	_integer_kind_end
	_unsigned_kind_start // uint
	TypeKindUint
	TypeKindUint8
	TypeKindUint16
	TypeKindUint32
	TypeKindUint64
	TypeKindUintptr
	_unsigned_kind_end
	TypeKindFloat32
	TypeKindFloat64
	_numeric_kind_end
	TypeKindComplex64
	TypeKindComplex128
	TypeKindString
	TypeKindUnsafePointer
	_basic_kind_end

	// non-basic
	TypeKindArray     // try to validate individual elements
	TypeKindInterface // try to validate ... ???
	TypeKindMap       // try to validate individual elements
	TypeKindPtr       // try to validate the element
	TypeKindSlice     // try to validate the individual elements
	TypeKindStruct    // try to validate the individual fields
	TypeKindChan      // don't validate
	TypeKindFunc      // don't validate

	// alisases (basic)
	TypeKindByte = TypeKindUint8
	TypeKindRune = TypeKindInt32
)

func (k TypeKind) IsBasic() bool { return _basic_kind_start < k && k < _basic_kind_end }

// Reports whether or not k is of the numeric kind, note that this
// does not include the complex64 and complex128 kinds.
func (k TypeKind) IsNumeric() bool { return _numeric_kind_start < k && k < _numeric_kind_end }

// Reports whether or not k is one of the int / uint types.
func (k TypeKind) IsInteger() bool { return _integer_kind_start < k && k < _integer_kind_end }

// Reports whether or not k is one of the uint types.
func (k TypeKind) IsUnsigned() bool { return _unsigned_kind_start < k && k < _unsigned_kind_end }

// Reports whether or not k is one of the float types.
func (k TypeKind) IsFloat() bool { return TypeKindFloat32 == k || k == TypeKindFloat64 }

// BasicString returns a string representation of k.
func (k TypeKind) BasicString() string {
	if k.IsBasic() {
		return typeKinds[k]
	}
	return "<unknown>"
}

func (k TypeKind) String() string {
	if int(k) < len(typeKinds) {
		return typeKinds[k]
	}
	return "<unknown>"
}

// Type kind string represenations indexed by typeKind.
var typeKinds = [...]string{
	TypeKindInvalid:    "<invalid>",
	TypeKindBool:       "bool",
	TypeKindInt:        "int",
	TypeKindInt8:       "int8",
	TypeKindInt16:      "int16",
	TypeKindInt32:      "int32",
	TypeKindInt64:      "int64",
	TypeKindUint:       "uint",
	TypeKindUint8:      "uint8",
	TypeKindUint16:     "uint16",
	TypeKindUint32:     "uint32",
	TypeKindUint64:     "uint64",
	TypeKindUintptr:    "uintptr",
	TypeKindFloat32:    "float32",
	TypeKindFloat64:    "float64",
	TypeKindComplex64:  "complex64",
	TypeKindComplex128: "complex128",
	TypeKindString:     "string",

	// ...
	TypeKindArray:     "array",
	TypeKindInterface: "interface",
	TypeKindMap:       "map",
	TypeKindPtr:       "ptr",
	TypeKindSlice:     "slice",
	TypeKindStruct:    "struct",
	TypeKindChan:      "chan",
	TypeKindFunc:      "func",
}

// typeKinds indexed by types.BasicKind.
var typesBasicKindToTypeKind = [...]TypeKind{
	types.Invalid:       TypeKindInvalid,
	types.Bool:          TypeKindBool,
	types.Int:           TypeKindInt,
	types.Int8:          TypeKindInt8,
	types.Int16:         TypeKindInt16,
	types.Int32:         TypeKindInt32,
	types.Int64:         TypeKindInt64,
	types.Uint:          TypeKindUint,
	types.Uint8:         TypeKindUint8,
	types.Uint16:        TypeKindUint16,
	types.Uint32:        TypeKindUint32,
	types.Uint64:        TypeKindUint64,
	types.Uintptr:       TypeKindUintptr,
	types.Float32:       TypeKindFloat32,
	types.Float64:       TypeKindFloat64,
	types.Complex64:     TypeKindComplex64,
	types.Complex128:    TypeKindComplex128,
	types.String:        TypeKindString,
	types.UnsafePointer: TypeKindUnsafePointer,
}
