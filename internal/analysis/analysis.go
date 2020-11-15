package analysis

import (
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/frk/isvalid/internal/search"
	"github.com/frk/tagutil"
)

// A Config specifies the configuration for the analysis.
type Config struct {
	// If set, specifies the struct tag to be used to produce a struct
	// field's key. If not set, the field's name will be used by default.
	FieldKeyTag string
	// If set to true, a nested struct field's key will be produced by
	// joining it together with all of its parent fields. If set to false,
	// such a field's key will be produced only from that field's name/tag.
	FieldKeyJoin bool
	// If set, specifies the separator that will be used when producing a
	// joined key for a nested struct field. If not set, the field's key
	// will be joined without a separator.
	//
	// This field is only used if FieldKeyJoin is set to true.
	FieldKeySeparator string

	// map of custom RuleSpecs
	ruleSpecMap map[string]RuleSpec
}

// AddRuleFunc is used to register a custom RuleFunc with the Config. The
// custom function MUST have at least one parameter item and, it MUST have
// exactly one result item which MUST be of type bool.
func (c *Config) AddRuleFunc(ruleName string, ruleFunc *types.Func) error {
	if name := strings.ToLower(ruleName); name == "isvalid" || name == "enum" {
		return &anError{Code: errRuleNameUnavailable}
	}

	sig := ruleFunc.Type().(*types.Signature)
	p, r := sig.Params(), sig.Results()
	if p.Len() < 1 || r.Len() != 1 {
		return &anError{Code: errRuleFuncParamCount}
	}
	if !isBool(r.At(0).Type()) {
		return &anError{Code: errRuleFuncResultType}
	}

	rf := RuleFunc{iscustom: true}
	rf.FuncName = ruleFunc.Name()
	rf.PkgPath = ruleFunc.Pkg().Path()
	rf.IsVariadic = sig.Variadic()
	for i := 0; i < p.Len(); i++ {
		rf.ArgTypes = append(rf.ArgTypes, analyzeType0(p.At(i).Type()))
	}

	if c.ruleSpecMap == nil {
		c.ruleSpecMap = make(map[string]RuleSpec)
	}
	c.ruleSpecMap[ruleName] = rf
	return nil
}

// Analyze runs the analysis of the validator struct represented by the given *search.Match.
// If successful, the returned *ValidatorStruct value is ready to be fed to the generator.
func (c Config) Analyze(ast search.AST, match *search.Match, info *Info) (*ValidatorStruct, error) {
	structType, ok := match.Named.Underlying().(*types.Struct)
	if !ok {
		panic(match.Named.Obj().Name() + " must be a struct type.") // this shouldn't happen
	}

	a := new(analysis)
	a.conf = c
	a.ast = ast
	a.fset = match.Fset
	a.named = match.Named
	a.pkgPath = match.Named.Obj().Pkg().Path()
	a.keys = make(map[string]uint)
	a.fieldVarMap = make(map[*StructField]fieldVar)

	a.info = info
	a.info.FileSet = match.Fset
	a.info.PkgPath = a.pkgPath
	a.info.TypeName = match.Named.Obj().Name()
	a.info.TypeNamePos = match.Pos
	a.info.SelectorMap = make(map[string]StructFieldSelector)
	a.info.EnumMap = make(map[string][]Const)

	a.fieldKey = fieldKeyFunc(c)
	vs, err := analyzeValidatorStruct(a, structType)
	if err != nil {
		return nil, err
	}

	// merge the rule func maps into one for the generator to use
	a.info.RuleSpecMap = make(map[string]RuleSpec)
	for k, v := range defaultRuleSpecMap {
		a.info.RuleSpecMap[k] = v
	}
	for k, v := range c.ruleSpecMap {
		a.info.RuleSpecMap[k] = v
	}

	return vs, nil
}

// Info holds information related to the analysis and inteded to be used by the generator.
// If the analysis returns an error, the collected information will be incomplete.
type Info struct {
	// The FileSet associated with the analyzed ValidatorStruct.
	FileSet *token.FileSet
	// The package path of the analyzed ValidatorStruct.
	PkgPath string
	// The type name of the analyzed ValidatorStruct.
	TypeName string
	// The soruce position of the ValidatorStruct's type name.
	TypeNamePos token.Pos
	// RuleSpecMap will be populated by all the registered RuleSpecs.
	RuleSpecMap map[string]RuleSpec
	// SelectorMap maps field keys to their related field selectors.
	SelectorMap map[string]StructFieldSelector
	// EnumMap maps package-path qualified type names to a slice of
	// constants declared with that type.
	EnumMap map[string][]Const
}

// analysis holds the state of the analyzer.
type analysis struct {
	conf Config
	// The AST as populated by search.Search.
	ast search.AST
	// The FileSet associated with the type under analysis,
	// used primarily for error reporting.
	fset *token.FileSet
	// The named type under analysis.
	named *types.Named
	// The package path of the type under analysis.
	pkgPath string
	// This field will hold the result of the analysis.
	validator *ValidatorStruct
	// Tracks already created field keys to ensure uniqueness.
	keys map[string]uint
	// Holds useful information aggregated during analysis.
	info *Info
	// Constructs a field key for the given selector, initialized from Config.
	fieldKey func([]*StructField) (key string)
	// For error reporting. If not nil it will hold the last encountered
	// rule & field that need the ValidatorStruct to have a "context" field.
	needsContext *needsContext
	// fieldVarMap maintains a map of StructField pointers to the fields'
	// related go/types specific information. Intended for error reporting.
	fieldVarMap map[*StructField]fieldVar
}

// used for error reporting only
type needsContext struct {
	field *StructField
	rule  *Rule
}

// used for error reporting only
type fieldVar struct {
	v   *types.Var
	tag string
}

// used by tests to trim away developer specific file system location of
// the project from testdata files' filepaths.
var filenamehook = func(name string) string { return name }

// anError wraps the given errorCode/*anError value with a number of
// detials that are intended to help with error reporting.
func (a *analysis) anError(e interface{}, f *StructField, r *Rule) error {
	var err *anError

	switch v := e.(type) {
	case errorCode:
		err = &anError{Code: v}
	case *anError:
		err = v
	default:
		panic("shouldn't reach")
	}

	if f != nil {
		err.FieldName = f.Name
		err.FieldTag = f.Tag
		err.FieldType = f.Type.String()
	}
	if r != nil {
		err.RuleName = r.Name
	}

	if fv, ok := a.fieldVarMap[f]; ok {
		pos := a.fset.Position(fv.v.Pos())
		err.FieldFileName = filenamehook(pos.Filename)
		err.FieldFileLine = pos.Line
		if f.Type.Kind == TypeKindInvalid {
			err.FieldType = fv.v.Type().String()
		}
	}

	obj := a.named.Obj()
	pos := a.fset.Position(obj.Pos())
	err.VtorName = obj.Name()
	err.VtorFileName = filenamehook(pos.Filename)
	err.VtorFileLine = pos.Line
	return err
}

// analyzeValidatorStruct is the entry point of a ValidatorStruct analysis.
func analyzeValidatorStruct(a *analysis, structType *types.Struct) (*ValidatorStruct, error) {
	a.validator = new(ValidatorStruct)
	a.validator.TypeName = a.named.Obj().Name()
	if name := lookupBeforeValidate(a.named); len(name) > 0 {
		a.validator.BeforeValidate = &MethodInfo{Name: name}
	}
	if name := lookupAfterValidate(a.named); len(name) > 0 {
		a.validator.AfterValidate = &MethodInfo{Name: name}
	}

	typName := strings.ToLower(a.validator.TypeName)
	if !strings.HasSuffix(typName, "validator") {
		panic(a.validator.TypeName + " struct type has unsupported name suffix.") // this shouldn't happen
	}

	// 1. analyze all fields
	fields, err := analyzeStructFields(a, structType, nil, true)
	if err != nil {
		return nil, err
	} else if len(fields) == 0 {
		return nil, a.anError(errEmptyValidator, nil, nil)
	}

	// 2. type-check all of the fields' rules
	if err := typeCheckRules(a, fields); err != nil {
		return nil, err
	}

	// 3. ensure that if a rule with context exists, that also a ContextOptionField exists
	if a.needsContext != nil && a.validator.ContextOption == nil {
		return nil, a.anError(errContextOptionFieldRequired, a.needsContext.field, a.needsContext.rule)
	}

	a.validator.Fields = fields
	return a.validator, nil
}

// analyzeStructFields analyzes the given *types.Struct's fields.
func analyzeStructFields(a *analysis, structType *types.Struct, selector []*StructField, local bool) (fields []*StructField, err error) {
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
		a.fieldVarMap[f] = fieldVar{v: fvar, tag: ftag}

		// resolve field key for selector & make sure that it is unique
		fsel := append(selector, f)
		f.Key = makeFieldKey(a, fsel)
		if _, ok := a.keys[f.Key]; ok {
			return nil, a.anError(errFieldKeyConflict, f, nil)
		} else {
			a.keys[f.Key] = 1
		}

		// map keys to selectors
		a.info.SelectorMap[f.Key] = append(StructFieldSelector{}, fsel...)

		maxdepth := 0
		typ, err := analyzeType(a, fvar.Type(), fsel, &maxdepth)
		if err != nil {
			return nil, err
		}
		f.Type = typ
		f.MaxFieldDepth = maxdepth

		if err := analyzeRules(a, f); err != nil {
			return nil, err
		}

		// Check for untagged, "special" root fields.
		if len(istag) == 0 && len(selector) == 0 {
			if isErrorConstructor(fvar.Type()) {
				if err := analyzeErrorHandlerField(a, f, false); err != nil {
					return nil, err
				}
				continue
			} else if isErrorAggregator(fvar.Type()) {
				if err := analyzeErrorHandlerField(a, f, true); err != nil {
					return nil, err
				}
				continue
			} else if strings.ToLower(fvar.Name()) == "context" {
				if err := analyzeContextOptionField(a, f); err != nil {
					return nil, err
				}
				continue
			}
		}

		fields = append(fields, f)
	}
	return fields, nil
}

// analyzeErrorHandlerField analyzes the given field as a ErrorHandlerField.
// The field's type is known to implement either the ErrorConstructor or the
// ErrorAggregator interface.
func analyzeErrorHandlerField(a *analysis, f *StructField, isAggregator bool) error {
	if a.validator.ErrorHandler != nil {
		return a.anError(errErrorHandlerFieldConflict, f, nil)
	}

	a.validator.ErrorHandler = new(ErrorHandlerField)
	a.validator.ErrorHandler.Name = f.Name
	a.validator.ErrorHandler.IsAggregator = isAggregator
	return nil
}

// analyzeContextOptionField analyzes the given field as a ContextOptionField.
// The field's name is known to be "context" (case insensitive).
func analyzeContextOptionField(a *analysis, f *StructField) error {
	if a.validator.ContextOption != nil {
		return a.anError(errContextOptionFieldConflict, f, nil)
	}
	if f.Type.Kind != TypeKindString {
		return a.anError(errContextOptionFieldType, f, nil)
	}

	a.validator.ContextOption = new(ContextOptionField)
	a.validator.ContextOption.Name = f.Name
	return nil
}

// analyzeType analyzes the given types.Type.
func analyzeType(a *analysis, t types.Type, selector []*StructField, maxdepth *int) (typ Type, err error) {
	if named, ok := t.(*types.Named); ok {
		pkg := named.Obj().Pkg()
		typ.Name = named.Obj().Name()
		typ.PkgPath = pkg.Path()
		typ.PkgName = pkg.Name()
		typ.PkgLocal = pkg.Name()
		typ.IsImported = isImportedType(a, named)
		typ.IsExported = named.Obj().Exported()
		typ.IsIsValider = isIsValider(t)
		t = named.Underlying()
	}

	typ.Kind = analyzeTypeKind(t)

	switch T := t.(type) {
	case *types.Basic:
		typ.IsRune = T.Name() == "rune"
		typ.IsByte = T.Name() == "byte"
	case *types.Slice:
		elem, err := analyzeType(a, T.Elem(), selector, maxdepth)
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &elem
	case *types.Array:
		elem, err := analyzeType(a, T.Elem(), selector, maxdepth)
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &elem
		typ.ArrayLen = T.Len()
	case *types.Map:
		key, err := analyzeType(a, T.Key(), selector, maxdepth)
		if err != nil {
			return Type{}, err
		}
		elem, err := analyzeType(a, T.Elem(), selector, maxdepth)
		if err != nil {
			return Type{}, err
		}
		typ.Key = &key
		typ.Elem = &elem
	case *types.Pointer:
		elem, err := analyzeType(a, T.Elem(), selector, maxdepth)
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &elem
	case *types.Interface:
		typ.IsEmptyInterface = T.NumMethods() == 0
		typ.IsIsValider = isIsValider(t)
	case *types.Struct:

		fields, err := analyzeStructFields(a, T, selector, !typ.IsImported)
		if err != nil {
			return Type{}, err
		}
		typ.Fields = fields

		// get the maxdepth
		if len(fields) > 0 {
			max := 0
			for _, f := range fields {
				if f.MaxFieldDepth > max {
					max = f.MaxFieldDepth
				}
			}
			*maxdepth = max + 1
		}
	}

	return typ, nil
}

// a simplified version of the above
func analyzeType0(t types.Type) (typ Type) {
	if named, ok := t.(*types.Named); ok {
		pkg := named.Obj().Pkg()
		typ.Name = named.Obj().Name()
		typ.PkgPath = pkg.Path()
		typ.PkgName = pkg.Name()
		typ.PkgLocal = pkg.Name()
		typ.IsExported = named.Obj().Exported()
		t = named.Underlying()
	}

	typ.Kind = analyzeTypeKind(t)

	switch T := t.(type) {
	case *types.Basic:
		typ.IsRune = T.Name() == "rune"
		typ.IsByte = T.Name() == "byte"
	case *types.Slice:
		elem := analyzeType0(T.Elem())
		typ.Elem = &elem
	case *types.Array:
		elem := analyzeType0(T.Elem())
		typ.Elem = &elem
		typ.ArrayLen = T.Len()
	case *types.Map:
		key := analyzeType0(T.Key())
		elem := analyzeType0(T.Elem())
		typ.Key = &key
		typ.Elem = &elem
	case *types.Pointer:
		elem := analyzeType0(T.Elem())
		typ.Elem = &elem
	case *types.Interface:
		typ.IsEmptyInterface = T.NumMethods() == 0
	case *types.Struct, *types.Chan:
		// TODO probably return an error
	}

	return typ
}

var rxInt = regexp.MustCompile(`^(?:0|-?[1-9][0-9]*)$`)
var rxFloat = regexp.MustCompile(`^(?:(?:-?0|[1-9][0-9]*)?\.[0-9]+)$`)
var rxBool = regexp.MustCompile(`^(?:false|true)$`)

// analyzeRules analyzes the rules of the given *StructField.
func analyzeRules(a *analysis, f *StructField) error {
	typ := f.Type.PtrBase()

	// first look for values that control the rest
	omitisvalid := false
	for _, s := range f.Tag["is"] {
		// The "-isvalid" tag value does not identify a specific rule,
		// instead it indicates that RuleIsValid should be voided.
		if strings.TrimSpace(s) == "-isvalid" {
			omitisvalid = true
		}
	}

	// Prepare the "isvalid" rule "backup" -- this will be added at the
	// end of the func if there are no empty rule names in the tag.
	var isvalid *Rule
	if typ.IsIsValider && !omitisvalid {
		isvalid = &Rule{Name: "isvalid"}
	}

	// do the rest...
	for _, s := range f.Tag["is"] {
		s = strings.TrimSpace(s)
		if s == "-isvalid" { // already handled above?
			continue
		}

		r := parseIsTagElem(s)
		if len(r.Context) > 0 && a.needsContext == nil {
			a.needsContext = &needsContext{f, r}
		}

		// handle "isvalid"
		switch r.Name {
		case "":
			if typ.IsIsValider && !omitisvalid {
				r.Name = "isvalid"

				// unset the isvalid variable so that it isn't
				// added a second time outside of the loop.
				isvalid = nil
			}
		case "isvalid":
			// The explicit "isvalid" tag value is actually not accepted,
			// instead the corresponding rule is inferred from the field's type.
			return a.anError(errRuleUnknown, f, r)
		}

		// make sure a spec is registered for the rule
		if !hasRuleSpec(a, r) {
			return a.anError(errRuleUnknown, f, r)
		}

		f.Rules = append(f.Rules, r)
	}

	// append "backup" if present
	if isvalid != nil {
		f.Rules = append(f.Rules, isvalid)
	}
	return nil
}

// hasRuleSpec reports whether or not the given *Rule has a RuleSpec registered.
func hasRuleSpec(a *analysis, r *Rule) bool {
	if r.Key != nil || r.Elem != nil {
		if r.Key != nil {
			if !hasRuleSpec(a, r.Key) {
				return false
			}
		}
		if r.Elem != nil {
			if !hasRuleSpec(a, r.Elem) {
				return false
			}
		}
		return true
	}

	if _, ok := a.conf.ruleSpecMap[r.Name]; !ok {
		if _, ok := defaultRuleSpecMap[r.Name]; !ok {
			return false
		}
	}
	return true
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

// makeFieldKey constructs a unique field key for the given selector.
func makeFieldKey(a *analysis, selector []*StructField) (key string) {
	key = a.fieldKey(selector)
	if num, ok := a.keys[key]; ok {
		a.keys[key] = num + 1
		key += "-" + strconv.FormatUint(uint64(num), 10)
	}
	return key
}

// isImportedType reports whether or not the given type is imported based on
// on the package in which the target of the analysis is declared.
func isImportedType(a *analysis, named *types.Named) bool {
	return named != nil && named.Obj().Pkg().Path() != a.pkgPath
}

// parses a single element of the `is` tag as a Rule and returns the result.
func parseIsTagElem(str string) *Rule {
	r := new(Rule)

	// do elem stuff?
	if len(str) > 0 && str[0] == '[' {
		// TODO handle quoted stuff
		if i := strings.IndexByte(str, ']'); i > -1 {
			str1 := str[1:i]
			str2 := str[i+1:]

			if len(str1) > 0 {
				r.Key = parseIsTagElem(str1)
			}
			if len(str2) > 0 {
				r.Elem = parseIsTagElem(str2)
			}
			return r
		} else {
			// TODO should fail or?
		}
	}

	opts := ""
	if i := strings.IndexByte(str, ':'); i > -1 {
		opts = str[i+1:]
		str = str[:i]
	}
	r.Name = str

	// if the opts string ends with ':' (e.g. `len:4:`) then append
	// an empty RuleArg to the end of the Rule.Args slice.
	var appendEmpty bool
	for len(opts) > 0 {

		var opt string
		if i := strings.IndexByte(opts, ':'); i > -1 {
			appendEmpty = (i == len(opts)-1) // is ':' the last char?
			opt = opts[:i]
			opts = opts[i+1:]
		} else {
			opt = opts
			opts = ""
		}

		var arg *RuleArg
		if len(opt) > 0 {
			switch opt[0] {
			case '#':
				r.SetKey = opt[1:]
			case '@':
				r.Context = opt[1:]
			case '&':
				arg = &RuleArg{Type: ArgTypeField}
				arg.Value = opt[1:]
			default:
				arg = &RuleArg{}
				arg.Value = opt

				// if value is surrounded by double quotes, remove both of them
				if n := len(arg.Value); n > 1 && arg.Value[0] == '"' && arg.Value[n-1] == '"' {
					arg.Value = arg.Value[1 : n-1]
					arg.Type = ArgTypeString
				} else {
					switch {
					case rxInt.MatchString(arg.Value):
						arg.Type = ArgTypeInt
					case rxFloat.MatchString(arg.Value):
						arg.Type = ArgTypeFloat
					case rxBool.MatchString(arg.Value):
						arg.Type = ArgTypeBool
					default:
						arg.Type = ArgTypeString
					}
				}
			}
		} else {
			arg = &RuleArg{}
		}

		if arg != nil {
			r.Args = append(r.Args, arg)
		}
	}

	if appendEmpty {
		r.Args = append(r.Args, &RuleArg{})
	}
	return r
}

// Checks all fields and their rules, and whether each rule can be applied
// to its related field without causing a compiler error.
func typeCheckRules(a *analysis, fields []*StructField) error {

	// walk recursively traverses the hierarchy of the given type and
	// invokes typeCheckRules on all struct fields it encounters.
	var walk func(a *analysis, typ Type) error
	walk = func(a *analysis, typ Type) error {
		typ = typ.PtrBase()
		switch typ.Kind {
		case TypeKindStruct:
			return typeCheckRules(a, typ.Fields)
		case TypeKindArray, TypeKindSlice:
			return walk(a, *typ.Elem)
		case TypeKindMap:
			if err := walk(a, *typ.Key); err != nil {
				return err
			}
			return walk(a, *typ.Elem)
		}
		return nil
	}

	for _, f := range fields {
		for _, r := range f.Rules {
			// Ensure that the Value of a RuleArg of type ArgTypeField
			// references a valid field key which will be indicated by
			// a presence of a selector in the SelectorMap.
			for _, arg := range r.Args {
				if arg.Type == ArgTypeField {
					if _, ok := a.info.SelectorMap[arg.Value]; !ok {
						// TODO test
						return a.anError(errFieldKeyUnknown, f, r)
					}
				}
			}

			if err := typeCheckRuleSpec(a, r, f.Type, f); err != nil {
				return err
			}
		}

		// TODO test type-checking for things like:
		// - (nested) slice-of-structs
		// - (nested) array-of-structs
		// - (nested) map-of-structs-to-structs
		if err := walk(a, f.Type); err != nil {
			return err
		}
	}
	return nil
}

// typeCheckRuleSpec looks up the RuleSpec for the given Rule and applies its
// corresponding type-checking to the provided Type. (f is used for error reporting)
func typeCheckRuleSpec(a *analysis, r *Rule, t Type, f *StructField) error {
	if r.Key != nil || r.Elem != nil {
		t = t.PtrBase()
		if r.Key != nil {
			if t.Kind != TypeKindMap {
				return a.anError(errTypeKind, f, r)
			}

			if err := typeCheckRuleSpec(a, r.Key, *t.Key, f); err != nil {
				return err
			}
		}
		if r.Elem != nil {
			if t.Kind != TypeKindArray && t.Kind != TypeKindSlice && t.Kind != TypeKindMap {
				return a.anError(errTypeKind, f, r)
			}

			if err := typeCheckRuleSpec(a, r.Elem, *t.Elem, f); err != nil {
				return err
			}
		}
		return nil
	}

	// Ensure a spec for the specified rule exists.
	spec, ok := a.conf.ruleSpecMap[r.Name]
	if !ok {
		spec, ok = defaultRuleSpecMap[r.Name]
		if !ok {
			return a.anError(errRuleUnknown, f, r)
		}
	}

	switch s := spec.(type) {
	case RuleIsValid:
		// do nothing
	case RuleEnum:
		if err := typeCheckRuleEnum(a, t, r, f); err != nil {
			return err
		}
	case RuleBasic:
		if err := s.check(a, r, t, f); err != nil {
			return err
		}
	case RuleFunc:
		if err := typeCheckRuleFunc(a, r, s, t, f); err != nil {
			return a.anError(err, f, r)
		}
	}

	return nil
}

// typeCheckRuleEnum checks whether the given Type can be used together
// with a RuleEnum. (r & f are used for error reporting)
func typeCheckRuleEnum(a *analysis, t Type, r *Rule, f *StructField) error {
	typ := t.PtrBase()
	if len(typ.Name) == 0 {
		return a.anError(errRuleEnumTypeUnnamed, f, r)
	}

	ident := typ.PkgPath + "." + typ.Name
	if _, ok := a.info.EnumMap[ident]; ok { // already done?
		return nil
	}

	enums := []Const{}
	consts := search.FindConstantsByType(typ.PkgPath, typ.Name, a.ast)
	for _, c := range consts {
		name := c.Name()
		pkgpath := c.Pkg().Path()
		// blank, skip
		if name == "_" {
			continue
		}
		// imported but not exported, skip
		if a.pkgPath != pkgpath && !c.Exported() {
			continue
		}
		enums = append(enums, Const{Name: name, PkgPath: pkgpath})
	}
	if len(enums) == 0 {
		return a.anError(errRuleEnumTypeNoConst, f, r)
	}

	a.info.EnumMap[ident] = enums
	return nil
}

// checks whether the rule func can be applied to its related field.
func typeCheckRuleFunc(a *analysis, r *Rule, rf RuleFunc, t Type, f *StructField) error {
	if rf.BoolConn > RuleFuncBoolNone {
		// func with bool connective but rule with no args, fail
		if len(r.Args) < 1 {
			return a.anError(errRuleFuncRuleArgCount, f, r)
		}
	} else {
		// rule arg count and func arg count are not compatibale, fail
		numreq := len(rf.ArgTypes[1:])
		if rf.IsVariadic {
			numreq -= 1
		}
		if numarg := len(r.Args); numreq > numarg || (numreq < numarg && !rf.IsVariadic) {
			return a.anError(errRuleFuncRuleArgCount, f, r)
		}
	}

	// field type cannot be converted to func 0th arg type, fail
	fldType, argType := t.PtrBase(), rf.ArgTypes[0]
	if rf.IsVariadic && len(rf.ArgTypes) == 1 {
		argType = *argType.Elem
	}
	if !canConvert(argType, fldType) {
		return a.anError(errRuleFuncFieldArgType, f, r)
	}

	// optional check returns error, fail
	if rf.check != nil {
		if err := rf.check(a, r, t, f); err != nil {
			return err
		}
	}

	// rule arg cannot be converted to func arg, fail
	fatypes := rf.ArgTypes[1:]
	if rf.IsVariadic && len(fatypes) > 0 {
		fatypes = fatypes[:len(fatypes)-1]
	}
	for i, fatyp := range fatypes {
		ra := r.Args[i]
		if !canConvertRuleArg(a, fatyp, ra) {
			return a.anError(&anError{Code: errRuleFuncRuleArgType, RuleArg: ra}, f, r)
		}
	}
	if rf.IsVariadic {
		fatyp := rf.ArgTypes[len(rf.ArgTypes)-1]
		fatyp = *fatyp.Elem
		for _, ra := range r.Args[len(fatypes):] {
			if !canConvertRuleArg(a, fatyp, ra) {
				return a.anError(&anError{Code: errRuleFuncRuleArgType, RuleArg: ra}, f, r)
			}
		}
	} else if rf.BoolConn > RuleFuncBoolNone {
		fatyp := rf.ArgTypes[1]
		for _, ra := range r.Args {
			if !canConvertRuleArg(a, fatyp, ra) {
				return a.anError(&anError{Code: errRuleFuncRuleArgType, RuleArg: ra}, f, r)
			}
		}
	}
	return nil
}

// canConvert reports whether src type can be converted to dst type. Note that
// this does not handle unnamed struct, interface, func, and channel types.
func canConvert(dst, src Type) bool {
	// if same, accept
	if src.Equals(dst) {
		return true
	}

	// if dst is interface{}, accept
	if dst.IsEmptyInterface {
		return true
	}

	// same basic kind, accept
	if dst.Kind == src.Kind && dst.Kind.IsBasic() {
		return true
	}

	// both numeric, accept
	if dst.Kind.IsNumeric() && src.Kind.IsNumeric() {
		return true
	}

	// string from []byte, []rune, []uint8, and []int32, accept
	if dst.Kind == TypeKindString && src.Kind == TypeKindSlice && src.Elem.Name == "" &&
		(src.Elem.Kind == TypeKindUint8 || src.Elem.Kind == TypeKindInt32) {
		return true
	}
	// string to []byte, []rune, []uint8, and []int32, accept
	if src.Kind == TypeKindString && dst.Kind == TypeKindSlice && dst.Elem.Name == "" &&
		(dst.Elem.Kind == TypeKindUint8 || dst.Elem.Kind == TypeKindInt32) {
		return true
	}

	// element types (and key & len) of non-basic are equal, accept
	if dst.Kind == src.Kind && !dst.Kind.IsBasic() {
		switch dst.Kind {
		case TypeKindArray:
			return dst.ArrayLen == src.ArrayLen && dst.Elem.Equals(*src.Elem)
		case TypeKindMap:
			return dst.Key.Equals(*src.Key) && dst.Elem.Equals(*src.Elem)
		case TypeKindSlice, TypeKindPtr:
			return dst.Elem.Equals(*src.Elem)
		}
	}
	return false
}

// canConvertRuleArg reports whether or not the src RuleArg's literal value
// can be converted to the type represented by dst.
func canConvertRuleArg(a *analysis, dst Type, src *RuleArg) bool {
	if src.Type == ArgTypeField {
		field := a.info.SelectorMap[src.Value].Last()
		return canConvert(dst, field.Type)
	}

	// dst is interface{} or string, accept
	if dst.IsEmptyInterface || dst.Kind == TypeKindString {
		return true
	}

	// src is unknown, accept
	if src.Type == ArgTypeUnknown {
		return true
	}

	// both are booleans, accept
	if dst.Kind == TypeKindBool && src.Type == ArgTypeBool {
		return true
	}

	// dst is float and arg is numeric, accept
	if dst.Kind.IsFloat() && (src.Type == ArgTypeInt || src.Type == ArgTypeFloat) {
		return true
	}

	// both are integers, accept
	if dst.Kind.IsInteger() && src.Type == ArgTypeInt {
		return true
	}

	// dst is unsigned and arg is not negative, accept
	if dst.Kind.IsUnsigned() && src.Type == ArgTypeInt && src.Value[0] != '-' {
		return true
	}

	// src is string & dst is convertable from string, accept
	if src.Type == ArgTypeString && (dst.Kind == TypeKindString || (dst.Kind == TypeKindSlice &&
		dst.Elem.Name == "" && (dst.Elem.Kind == TypeKindUint8 || dst.Elem.Kind == TypeKindInt32))) {
		return true
	}

	return false
}

// fieldKeyFunc returns a function that, based on the given config, produces
// field keys from a list of struct fields.
func fieldKeyFunc(conf Config) (fn func([]*StructField) string) {
	if len(conf.FieldKeyTag) > 0 {
		if conf.FieldKeyJoin {
			tag := conf.FieldKeyTag
			sep := conf.FieldKeySeparator
			// Returns the joined tag values of the fields in the given slice.
			// If one of the fields does not have a tag value set, their name
			// will be used in the join as default.
			return func(sel []*StructField) (key string) {
				for _, f := range sel {
					if f.Tag.Contains("isvalid", "omitkey") {
						continue
					}

					v := f.Tag.First(tag)
					if len(v) == 0 {
						v = f.Name
					}
					key += v + sep
				}
				if len(sep) > 0 && len(key) > len(sep) {
					return key[:len(key)-len(sep)]
				}
				return key
			}
		}

		// Returns the tag value of the last field, if no value was
		// set the field's name will be returned instead.
		return func(sel []*StructField) string {
			if key := sel[len(sel)-1].Tag.First(conf.FieldKeyTag); len(key) > 0 {
				return key
			}
			return sel[len(sel)-1].Name
		}
	}

	if conf.FieldKeyJoin {
		sep := conf.FieldKeySeparator
		// Returns the joined names of the fields in the given slice.
		return func(sel []*StructField) (key string) {
			for _, f := range sel {
				if f.Tag.Contains("isvalid", "omitkey") {
					continue
				}
				key += f.Name + sep
			}
			if len(sep) > 0 && len(key) > len(sep) {
				return key[:len(key)-len(sep)]
			}
			return key
		}
	}

	// Returns the name of the last field.
	return func(sel []*StructField) string {
		return sel[len(sel)-1].Name
	}
}
