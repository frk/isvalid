package analysis

import (
	"fmt"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/frk/tagutil"
)

var _ = fmt.Println

// Info holds information related to an analyzed ValidatorStruct. If the analysis
// returns an error, the collected information will be incomplete.
type Info struct {
	// The FileSet associated with the analyzed ValidatorStruct.
	FileSet *token.FileSet
	// The package path of the analyzed ValidatorStruct.
	PkgPath string
	// The type name of the analyzed ValidatorStruct.
	TypeName string
	// The soruce position of the ValidatorStruct's type name.
	TypeNamePos token.Pos
	// FieldMap maintains a map of StructField pointers to the fields'
	// related go/types specific information. Intended for error reporting.
	FieldMap map[*StructField]FieldVar
	// Maps RuleArgs of kind ArgKindReference to information
	// that's used by the analysis and by the generator.
	ArgReferenceMap map[*RuleArg]*ArgReferenceInfo
}

// analysis holds the state of the analyzer.
type analysis struct {
	fset *token.FileSet
	// The named type under analysis.
	named *types.Named
	// The package path of the type under analysis.
	pkgPath string
	// This field will hold the result of the analysis.
	validator *ValidatorStruct
	// The selector of the current field under analysis.
	selector []*StructField
	// Tracks already created field keys to ensure uniqueness.
	keys map[string]uint
	// Holds useful information aggregated during analysis.
	info *Info
}

func (a *analysis) anError(e interface{}, f *StructField, r *Rule) error {
	var err *anError

	switch v := e.(type) {
	case errorCode:
		err = &anError{Code: v}
	case *anError:
		err = v
	}

	if f != nil {
		err.FieldName = f.Name
		err.FieldTag = f.Tag
		err.FieldType = f.Type.String()
	}
	if r != nil {
		err.RuleName = r.Name
	}

	if fv, ok := a.info.FieldMap[f]; ok {
		pos := a.fset.Position(fv.Var.Pos())
		err.FieldFileName = pos.Filename
		err.FieldFileLine = pos.Line
	}

	obj := a.named.Obj()
	pos := a.fset.Position(obj.Pos())
	err.VtorName = obj.Name()
	err.VtorFileName = pos.Filename
	err.VtorFileLine = pos.Line
	return err
}

func Run(fset *token.FileSet, named *types.Named, pos token.Pos, info *Info) (*ValidatorStruct, error) {
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		panic(named.Obj().Name() + " must be a struct type.") // this shouldn't happen
	}

	a := new(analysis)
	a.fset = fset
	a.named = named
	a.pkgPath = named.Obj().Pkg().Path()
	a.keys = make(map[string]uint)

	a.info = info
	a.info.FileSet = fset
	a.info.PkgPath = a.pkgPath
	a.info.TypeName = named.Obj().Name()
	a.info.TypeNamePos = pos
	a.info.FieldMap = make(map[*StructField]FieldVar)
	a.info.ArgReferenceMap = make(map[*RuleArg]*ArgReferenceInfo)

	return analyzeValidatorStruct(a, structType)
}

// analyzeValidatorStruct runs the analysis of a ValidatorStruct.
func analyzeValidatorStruct(a *analysis, structType *types.Struct) (*ValidatorStruct, error) {
	a.validator = new(ValidatorStruct)
	a.validator.TypeName = a.named.Obj().Name()

	typName := strings.ToLower(a.validator.TypeName)
	if !strings.HasSuffix(typName, "validator") {
		panic(a.validator.TypeName + " struct type has unsupported name suffix.") // this shouldn't happen
	}

	// 1. analyze all fields
	fields, err := analyzeStructFields(a, structType, true, true)
	if err != nil {
		return nil, err
	} else if len(fields) == 0 {
		return nil, a.anError(errEmptyValidator, nil, nil)
	}

	// 2. resolve any arg references
	for p, info := range a.info.ArgReferenceMap {
		info.Selector = findSelectorForKey(p.Value, fields)
		if len(info.Selector) == 0 {
			return nil, a.anError(errFieldKeyUnknown, info.StructField, info.Rule)
		}
	}

	// 3. type-check all of the fields' rules
	if err := ruleCheckStructFields(a, fields); err != nil {
		return nil, err
	}

	a.validator.Fields = fields
	return a.validator, nil
}

func analyzeStructFields(a *analysis, structType *types.Struct, root bool, local bool) (fields []*StructField, err error) {
	for i := 0; i < structType.NumFields(); i++ {
		fvar := structType.Field(i)
		ftag := structType.Tag(i)
		tag := tagutil.New(ftag)
		istag := tag.First("is")

		// Skip imported but unexported fields.
		if !local && !fvar.Exported() {
			continue
		}

		// Skip fields with blank name.
		if fvar.Name() == "_" {
			continue
		}

		// Skip fields that were explicitly flagged; fields with
		// no `is` tag may still be useful if they are referenced by
		// a separate field's rule with a reference arg.
		if istag == "-" {
			continue
		}

		f := new(StructField)
		f.Tag = tag
		f.Name = fvar.Name()
		f.IsEmbedded = fvar.Embedded()
		f.IsExported = fvar.Exported()

		// map field to fvar for error reporting
		a.info.FieldMap[f] = FieldVar{Var: fvar, Tag: ftag}

		// set the selector for the current field,
		// used by key resolution and error reporting
		if root {
			a.selector = []*StructField{f}
		} else {
			a.selector = append(a.selector, f)
		}

		// resolve field key & make sure that it is unique
		f.Key = makeFieldKey(a, fvar, ftag)
		if _, ok := a.keys[f.Key]; ok {
			return nil, a.anError(errFieldKeyConflict, f, nil)
		} else {
			a.keys[f.Key] = 1
		}

		typ, err := analyzeType(a, fvar.Type())
		if err != nil {
			return nil, err
		}
		f.Type = typ

		if len(istag) > 0 {
			if err := analyzeRules(a, f); err != nil {
				return nil, err
			}
		}

		fields = append(fields, f)
	}
	return fields, nil
}

func analyzeType(a *analysis, t types.Type) (typ Type, err error) {
	if named, ok := t.(*types.Named); ok {
		pkg := named.Obj().Pkg()
		typ.Name = named.Obj().Name()
		typ.PkgPath = pkg.Path()
		typ.PkgName = pkg.Name()
		typ.PkgLocal = pkg.Name()
		typ.IsImported = isImportedType(a, named)
		typ.IsExported = named.Obj().Exported()
		t = named.Underlying()
	}

	typ.Kind = analyzeTypeKind(t)

	switch T := t.(type) {
	case *types.Basic:
		typ.IsRune = T.Name() == "rune"
		typ.IsByte = T.Name() == "byte"
	case *types.Slice:
		elem, err := analyzeType(a, T.Elem())
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &elem
	case *types.Array:
		elem, err := analyzeType(a, T.Elem())
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &elem
		typ.ArrayLen = T.Len()
	case *types.Map:
		key, err := analyzeType(a, T.Key())
		if err != nil {
			return Type{}, err
		}
		elem, err := analyzeType(a, T.Elem())
		if err != nil {
			return Type{}, err
		}
		typ.Key = &key
		typ.Elem = &elem
	case *types.Pointer:
		elem, err := analyzeType(a, T.Elem())
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &elem
	case *types.Interface:
		typ.IsEmptyInterface = T.NumMethods() == 0
	case *types.Struct:
		fields, err := analyzeStructFields(a, T, false, !typ.IsImported)
		if err != nil {
			return Type{}, err
		}
		typ.Fields = fields
	}

	return typ, nil
}

var rxNint = regexp.MustCompile(`^(?:-[1-9][0-9]*)$`) // negative integer
var rxUint = regexp.MustCompile(`^(?:0|[1-9][0-9]*)$`)
var rxFloat = regexp.MustCompile(`^(?:(?:-?0|[1-9][0-9]*)?\.[0-9]+)$`)

func analyzeRules(a *analysis, f *StructField) error {
	for _, s := range f.Tag["is"] {
		r := parseRule(s)
		for _, p := range r.Args {
			if p.Kind == ArgKindReference {
				a.info.ArgReferenceMap[p] = &ArgReferenceInfo{
					Rule:        r,
					StructField: f,
				}
			}
		}

		// check that rule type exists
		if _, err := ruleTypes.find(r); err != nil {
			return a.anError(err, f, r)
		}

		f.Rules = append(f.Rules, r)
	}
	return nil
}

// analyzeTypeKind returns the TypeKind for the given types.Type.
func analyzeTypeKind(typ types.Type) TypeKind {
	switch x := typ.(type) {
	case *types.Basic:
		return typesBasicKindToTypeKind[x.Kind()]
	case *types.Array:
		return TypeKindArray
	case *types.Chan:
		return TypeKindChan
	case *types.Signature:
		return TypeKindFunc
	case *types.Interface:
		return TypeKindInterface
	case *types.Map:
		return TypeKindMap
	case *types.Pointer:
		return TypeKindPtr
	case *types.Slice:
		return TypeKindSlice
	case *types.Struct:
		return TypeKindStruct
	case *types.Named:
		return analyzeTypeKind(x.Underlying())
	}
	return 0 // unsupported / unknown
}

func makeFieldKey(a *analysis, fvar *types.Var, ftag string) string {
	// TODO
	// The key of the StructField (used for errors, reference args, etc.),
	// the value of this is determined by the "f key" setting, if not
	// specified by the user it will default to the value of the f's name.

	// default strategy
	k := fvar.Name()
	if num, ok := a.keys[k]; ok {
		k += "-" + strconv.FormatUint(uint64(num), 10)
		a.keys[k] = num + 1

	}
	return k
}

// isImportedType reports whether or not the given type is imported based on
// on the package in which the target of the analysis is declared.
func isImportedType(a *analysis, named *types.Named) bool {
	return named != nil && named.Obj().Pkg().Path() != a.pkgPath
}

// expected format
// rule_name{ :rule_arg }
func parseRule(str string) *Rule {
	str = strings.TrimSpace(str)
	name := str
	args := ""

	if i := strings.IndexByte(str, ':'); i > -1 {
		name = str[:i]
		args = str[i+1:]
	}

	// if the args string ends with ':' (e.g. `len:4:`) then append
	// an empty RuleArg to the end of the Rule.Args slice.
	var appendEmpty bool

	r := &Rule{Name: name}
	for len(args) > 0 {
		a := &RuleArg{}
		if i := strings.IndexByte(args, ':'); i > -1 {
			appendEmpty = (i == len(args)-1) // is ':' the last char?
			a.Value = args[:i]
			args = args[i+1:]
		} else {
			a.Value = args
			args = ""
		}

		if len(a.Value) > 0 {
			switch a.Value[0] {
			case '&':
				a.Kind = ArgKindReference
				a.Value = a.Value[1:]
			case '#':
				a.Kind = ArgKindGroupKey
				a.Value = a.Value[1:]
			case '@':
				a.Kind = ArgKindContext
				a.Value = a.Value[1:]
			default:
				a.Kind = ArgKindLiteral

				// if value is surrounded by double quotes, remove both of them
				if n := len(a.Value); n > 1 && a.Value[0] == '"' && a.Value[n-1] == '"' {
					a.Value = a.Value[1 : n-1]
					a.Type = ArgTypeString
				}
			}

			if a.Kind == ArgKindLiteral && a.Type == 0 {
				switch {
				case rxNint.MatchString(a.Value):
					a.Type = ArgTypeNint
				case rxUint.MatchString(a.Value):
					a.Type = ArgTypeUint
				case rxFloat.MatchString(a.Value):
					a.Type = ArgTypeFloat
				default:
					a.Type = ArgTypeString
				}
			}
		}

		r.Args = append(r.Args, a)
	}

	if appendEmpty {
		r.Args = append(r.Args, &RuleArg{})
	}
	return r
}

func ruleCheckStructFields(a *analysis, fields []*StructField) error {
	for _, f := range fields {
		for _, r := range f.Rules {
			if err := ruleTypes.check(a, f, r); err != nil {
				return a.anError(err, f, r)
			}
		}

		if f.Type.Kind == TypeKindStruct {
			if err := ruleCheckStructFields(a, f.Type.Fields); err != nil {
				return err
			}
		}
		if f.Type.Kind == TypeKindPtr && f.Type.Elem.Kind == TypeKindStruct {
			if err := ruleCheckStructFields(a, f.Type.Elem.Fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func findSelectorForKey(key string, fields []*StructField) []*StructField {
	for _, f := range fields {
		if f.Key == key {
			return []*StructField{f}
		}
		if f.Type.Kind == TypeKindStruct {
			if s := findSelectorForKey(key, f.Type.Fields); len(s) > 0 {
				return append([]*StructField{f}, s...)
			}
		}
		if f.Type.Kind == TypeKindPtr && f.Type.Elem.Kind == TypeKindStruct {
			if s := findSelectorForKey(key, f.Type.Elem.Fields); len(s) > 0 {
				return append([]*StructField{f}, s...)
			}
		}
	}
	return nil
}
