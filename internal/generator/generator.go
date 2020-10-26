package generator

import (
	"io"
	"log"
	"strings"

	"github.com/frk/isvalid/internal/analysis"

	GO "github.com/frk/ast/golang"
)

var _ = log.Println

const (
	isvalidPkgPath = `github.com/frk/isvalid`

	filePreamble = ` DO NOT EDIT. This file was generated by "github.com/frk/isvalid".`
)

type Config struct {
	// ...
}

type TargetInfo struct {
	ValidatorStruct *analysis.ValidatorStruct
	Info            *analysis.Info
}

func Write(f io.Writer, pkgName string, targets []*TargetInfo, conf Config) error {
	var (
		file          = new(GO.File)
		imports       = new(GO.ImportDecl)
		importIsvalid bool
		importStrings bool
	)
	for _, t := range targets {
		g := new(generator)
		g.conf = conf
		g.info = t.Info
		g.vs = t.ValidatorStruct
		g.file = file
		g.imports = imports

		buildCode(g, t.ValidatorStruct)
		if g.importIsvalid {
			importIsvalid = true
		}
		if g.importStrings {
			importStrings = true
		}
	}

	imports.Specs = append(imports.Specs, GO.ImportSpec{Path: "errors"})

	if importIsvalid {
		imports.Specs = append(imports.Specs, GO.ImportSpec{Path: isvalidPkgPath})
	}
	if importStrings {
		imports.Specs = append(imports.Specs, GO.ImportSpec{Path: "strings"})
	}
	sortImports(imports)

	file.PkgName = pkgName
	file.Preamble = GO.LineComment{filePreamble}
	file.Imports = []GO.ImportDeclNode{imports}
	return GO.Write(file, f)
}

type generator struct {
	// The generator configuration.
	conf Config
	info *analysis.Info
	vs   *analysis.ValidatorStruct
	init []GO.StmtNode
	// The target file to be build by the generator.
	file *GO.File
	// The associated file's import declaration.
	imports       *GO.ImportDecl
	importIsvalid bool
	importStrings bool
}

func buildCode(g *generator, vs *analysis.ValidatorStruct) {
	root := GO.Ident{"v"}
	body := []GO.StmtNode{}
	for _, f := range vs.Fields {
		buildFieldCode(g, f, root, &body)
	}
	body = append(body, GO.ReturnStmt{GO.Ident{"nil"}})

	if len(g.init) > 0 {
		init := GO.FuncDecl{}
		init.Name.Name = "init"
		init.Body.List = g.init
		g.file.Decls = append(g.file.Decls, init)
	}

	method := GO.MethodDecl{}
	method.Recv.Name = root
	method.Recv.Type = GO.Ident{vs.TypeName}
	method.Name.Name = "Validate"
	method.Type.Results = GO.ParamList{{Type: GO.Ident{"error"}}}
	method.Body.List = body

	g.file.Decls = append(g.file.Decls, method)
}

func buildFieldCode(g *generator, field *analysis.StructField, root GO.ExprNode, body *[]GO.StmtNode) {
	rules := field.RulesCopy()
	subfields := field.SubFields()
	if len(rules) == 0 && len(subfields) == 0 { // nothing to do?
		return
	}

	fieldExpr := GO.ExprNode(GO.SelectorExpr{X: root, Sel: GO.Ident{field.Name}})

	// special case: no if-stmt necessary for this field, but possibly subfields
	if field.Type.Kind != analysis.TypeKindPtr && len(rules) == 0 && len(subfields) > 0 {
		root := GO.Ident{"f"}
		block := []GO.StmtNode{}
		for _, f := range subfields {
			buildFieldCode(g, f, root, &block)
		}

		if len(block) > 0 {
			assign := GO.AssignStmt{Token: GO.AssignDefine, Lhs: root, Rhs: fieldExpr}
			block = append([]GO.StmtNode{assign}, block...)

			*body = append(*body, GO.BlockStmt{block})
		}
		return
	}

	var required, notnil *analysis.Rule
	for i := 0; i < len(rules); i++ {
		if r := rules[i]; r.Name == "required" || r.Name == "notnil" {
			if r.Name == "required" {
				required = r
			} else if r.Name == "notnil" {
				notnil = r
			}

			// delete (from https://github.com/golang/go/wiki/SliceTricks)
			copy(rules[i:], rules[i+1:])
			rules[len(rules)-1] = nil
			rules = rules[:len(rules)-1]

		}
	}

	ifs := GO.IfStmt{}
	nilId := GO.Ident{"nil"}
	fieldType := field.Type

	if fieldType.Kind == analysis.TypeKindPtr {
		// logical op, and equality op
		lop, eop := GO.BinaryLAnd, GO.BinaryNeq
		if required != nil || notnil != nil {
			lop, eop = GO.BinaryLOr, GO.BinaryEql
		}

		ifs.Cond = GO.BinaryExpr{Op: eop, X: fieldExpr, Y: nilId}
		fieldExpr = GO.PointerIndirectionExpr{fieldExpr}
		fieldType = *fieldType.Elem

		// handle multiple pointers
		for fieldType.Kind == analysis.TypeKindPtr {
			ifs.Cond = GO.BinaryExpr{Op: lop, X: ifs.Cond, Y: GO.BinaryExpr{Op: eop, X: fieldExpr, Y: nilId}}
			fieldExpr = GO.PointerIndirectionExpr{fieldExpr}
			fieldType = *fieldType.Elem
		}
	}

	if required != nil || notnil != nil {
		var ruleExpr GO.ExprNode
		var retStmt GO.StmtNode
		if required != nil {
			ruleExpr = makeExprForRequired(g, fieldType.Kind, fieldExpr)
			retStmt = makeReturnStmtForError(g, field.Key+" is required")
		} else if notnil != nil {
			ruleExpr = makeExprForNotnil(g, fieldType.Kind, fieldExpr)
			retStmt = makeReturnStmtForError(g, field.Key+" cannot be nil")
		}

		if ruleExpr != nil {
			if ifs.Cond != nil {
				ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLOr, X: ifs.Cond, Y: ruleExpr}
			} else {
				ifs.Cond = ruleExpr
			}
		}

		if retStmt != nil {
			ifs.Body.Add(retStmt)
		}
	}

	if len(rules) > 0 || len(subfields) > 0 {
		block := []GO.StmtNode{}
		if ifs.Cond != nil {
			fieldVar := GO.Ident{"f"}
			assign := GO.AssignStmt{Token: GO.AssignDefine, Lhs: fieldVar, Rhs: fieldExpr}
			block = append(block, assign)

			fieldExpr = fieldVar
		}

		if len(rules) > 0 {
			mainIf, elseIf := GO.IfStmt{}, (*GO.IfStmt)(nil)
			for _, r := range rules {
				ifs := ruleIfStmtMap[r.Name](g, r, field, fieldExpr)
				if elseIf != nil {
					elseIf.Else = &ifs
					elseIf = &ifs
				} else if mainIf.Cond != nil {
					mainIf.Else = &ifs
					elseIf = &ifs
				} else {
					mainIf = ifs
				}
			}

			if ifs.Cond != nil {
				block = append(block, mainIf)
			} else {
				ifs = mainIf
			}
		}

		// loop over subfields
		if len(subfields) > 0 {
			for _, f := range subfields {
				buildFieldCode(g, f, fieldExpr, &block)
			}
		}

		if required != nil || notnil != nil {
			ifs.Else = GO.BlockStmt{block}
		} else {
			ifs.Body.Add(block...)
		}
	}

	*body = append(*body, ifs)
}

func makeExprForRequired(g *generator, kind analysis.TypeKind, fieldExpr GO.ExprNode) GO.ExprNode {
	switch kind {
	case analysis.TypeKindString, analysis.TypeKindMap, analysis.TypeKindSlice:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: GO.CallLenExpr{fieldExpr}, Y: GO.IntLit(0)}
	case analysis.TypeKindInt, analysis.TypeKindInt8, analysis.TypeKindInt16, analysis.TypeKindInt32, analysis.TypeKindInt64:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: GO.IntLit(0)}
	case analysis.TypeKindUint, analysis.TypeKindUint8, analysis.TypeKindUint16, analysis.TypeKindUint32, analysis.TypeKindUint64:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: GO.IntLit(0)}
	case analysis.TypeKindFloat32, analysis.TypeKindFloat64:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: GO.ValueLit("0.0")}
	case analysis.TypeKindBool:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: GO.ValueLit("false")}
	case analysis.TypeKindInterface:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: GO.Ident{"nil"}}
	}
	return nil
}

func makeExprForNotnil(g *generator, kind analysis.TypeKind, fieldExpr GO.ExprNode) GO.ExprNode {
	switch kind {
	case analysis.TypeKindInterface, analysis.TypeKindMap, analysis.TypeKindSlice:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: GO.Ident{"nil"}}
	}
	return nil
}

var ruleIfStmtMap = map[string]func(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) GO.IfStmt{
	"email":    ifStmtMaker("Email", "must be a valid email"),
	"url":      ifStmtMaker("URL", "must be a valid URL"),
	"uri":      ifStmtMaker("URI", "must be a valid URI"),
	"pan":      ifStmtMaker("PAN", "must be a valid PAN"),
	"cvv":      ifStmtMaker("CVV", "must be a valid CVV"),
	"ssn":      ifStmtMaker("SSN", "must be a valid SSN"),
	"ein":      ifStmtMaker("EIN", "must be a valid EIN"),
	"numeric":  ifStmtMaker("Numeric", "must contain only digits [0-9]"),
	"hex":      ifStmtMaker("Hex", "must be a valid hexadecimal string"),
	"hexcolor": ifStmtMaker("HexColor", "must be a valid hex color code"),
	"alphanum": ifStmtMaker("Alphanum", "must be an alphanumeric string"),
	"cidr":     ifStmtMaker("CIDR", "must be a valid CIDR"),
	"phone":    makeIfStmtForPhone,
	"zip":      makeIfStmtForZip,
	"uuid":     makeIfStmtForUUID,
	"ip":       makeIfStmtForIP,
	"mac":      makeIfStmtForMAC,
	"iso":      makeIfStmtForISO,
	"rfc":      makeIfStmtForRFC,
	"re":       makeIfStmtForRegexp,
	"prefix":   makeIfStmtForPrefix,
	"suffix":   makeIfStmtForSuffix,
	"contains": makeIfStmtForContains,
	"eq":       makeIfStmtForEquals,
	"ne":       makeIfStmtForNotEquals,
}

func ifStmtMaker(funcName string, errMessage string) (maker func(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt)) {
	return func(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
		g.importIsvalid = true
		fn := GO.QualifiedIdent{"isvalid", funcName}
		call := GO.CallExpr{Fun: fn, Args: GO.ArgsList{List: fieldExpr}}
		retStmt := makeReturnStmtForError(g, field.Key+" "+errMessage)

		ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
		ifs.Body.Add(retStmt)
		return ifs
	}
}

func makeIfStmtForPhone(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "Phone"}
	call := GO.CallExpr{Fun: fn}
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid phone number")

	args := GO.ExprList{fieldExpr}
	for _, a := range r.Args {
		args = append(args, GO.StringLit(a.Value))
	}
	call.Args.List = args

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForZip(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "Zip"}
	call := GO.CallExpr{Fun: fn}
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid zip code")

	args := GO.ExprList{fieldExpr}
	for _, a := range r.Args {
		args = append(args, GO.StringLit(a.Value))
	}
	call.Args.List = args

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForUUID(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "UUID"}
	call := GO.CallExpr{Fun: fn}
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid UUID")

	args := GO.ExprList{fieldExpr}
	for _, a := range r.Args {
		// if analysis did its job correctly a.Value will be either a
		// single digit integer or a "v<DIGIT>" string, in the latter
		// case remove the "v" so we can pass the argument as int.
		v := a.Value
		if len(v) > 1 {
			v = v[1:]
		}
		args = append(args, GO.ValueLit(v))
	}
	call.Args.List = args

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForIP(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "IP"}
	call := GO.CallExpr{Fun: fn}
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid IP")

	args := GO.ExprList{fieldExpr}
	for _, a := range r.Args {
		// if analysis did its job correctly a.Value will be either a
		// single digit integer or a "v<DIGIT>" string, in the latter
		// case remove the "v" so we can pass the argument as int.
		v := a.Value
		if len(v) > 1 {
			v = v[1:]
		}
		args = append(args, GO.ValueLit(v))
	}
	call.Args.List = args

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForMAC(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "MAC"}
	call := GO.CallExpr{Fun: fn}
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid MAC")

	args := GO.ExprList{fieldExpr}
	for _, a := range r.Args {
		// if analysis did its job correctly a.Value will be either a
		// single digit integer or a "v<DIGIT>" string, in the latter
		// case remove the "v" so we can pass the argument as int.
		v := a.Value
		if len(v) > 1 {
			v = v[1:]
		}
		args = append(args, GO.ValueLit(v))
	}
	call.Args.List = args

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForISO(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "ISO"}
	call := GO.CallExpr{Fun: fn}

	// if analysis did its job correctly r.Args will be of len 1
	// and the argument will represent an integer.
	arg := r.Args[0].Value
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid ISO "+arg)
	call.Args.List = GO.ExprList{fieldExpr, GO.ValueLit(arg)}

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForRFC(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "RFC"}
	call := GO.CallExpr{Fun: fn}

	// if analysis did its job correctly r.Args will be of len 1
	// and the argument will represent an integer.
	arg := r.Args[0].Value
	retStmt := makeReturnStmtForError(g, field.Key+" must be a valid RFC "+arg)
	call.Args.List = GO.ExprList{fieldExpr, GO.ValueLit(arg)}

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForRegexp(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importIsvalid = true
	fn := GO.QualifiedIdent{"isvalid", "Match"}
	call := GO.CallExpr{Fun: fn}

	// if analysis did its job correctly r.Args will be of len 1
	// and the argument will represent a regular expression.
	arg := r.Args[0].Value
	retStmt := makeReturnStmtForErrorRaw(g, field.Key+" must match the regular expression: "+arg)
	call.Args.List = GO.ExprList{fieldExpr, GO.RawStringLit(arg)}

	// add a registry call for the init function
	regrx := GO.CallExpr{Fun: GO.QualifiedIdent{"isvalid", "RegisterRegexp"}}
	regrx.Args.List = GO.RawStringLit(arg)
	g.init = append(g.init, GO.ExprStmt{regrx})

	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForPrefix(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importStrings = true

	args := make([]string, len(r.Args)) // for error message
	for i, a := range r.Args {
		args[i] = `\"` + a.Value + `\"`

		call := GO.CallExpr{Fun: GO.QualifiedIdent{"strings", "HasPrefix"}}
		call.Args.List = GO.ExprList{fieldExpr, GO.StringLit(a.Value)}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: GO.UnaryExpr{Op: GO.UnaryNot, X: call}}
		} else {
			ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
		}
	}

	retStmt := makeReturnStmtForError(g, field.Key+" must be prefixed with: "+strings.Join(args, " or "))

	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForSuffix(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importStrings = true

	args := make([]string, len(r.Args)) // for error message
	for i, a := range r.Args {
		args[i] = `\"` + a.Value + `\"`

		call := GO.CallExpr{Fun: GO.QualifiedIdent{"strings", "HasSuffix"}}
		call.Args.List = GO.ExprList{fieldExpr, GO.StringLit(a.Value)}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: GO.UnaryExpr{Op: GO.UnaryNot, X: call}}
		} else {
			ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
		}
	}

	retStmt := makeReturnStmtForError(g, field.Key+" must be suffixed with: "+strings.Join(args, " or "))

	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForContains(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	g.importStrings = true

	args := make([]string, len(r.Args)) // for error message
	for i, a := range r.Args {
		args[i] = `\"` + a.Value + `\"`

		call := GO.CallExpr{Fun: GO.QualifiedIdent{"strings", "Contains"}}
		call.Args.List = GO.ExprList{fieldExpr, GO.StringLit(a.Value)}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: GO.UnaryExpr{Op: GO.UnaryNot, X: call}}
		} else {
			ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
		}
	}

	retStmt := makeReturnStmtForError(g, field.Key+" must contain substring: "+strings.Join(args, " or "))

	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForEquals(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	typ := field.Type
	for typ.Kind == analysis.TypeKindPtr {
		typ = *typ.Elem
	}

	args := make([]string, len(r.Args)) // for error message
	for i, a := range r.Args {
		val := a.Value

		var y GO.ExprNode
		if typ.Kind == analysis.TypeKindString {
			y = GO.StringLit(val)
		} else if a.Type == analysis.ArgTypeString {
			if typ.Kind.IsNumeric() && len(val) == 0 {
				y = GO.IntLit(0)
				val = "0"
			} else {
				y = GO.StringLit(val)
			}
		} else {
			y = GO.ValueLit(val)
		}

		cond := GO.BinaryExpr{Op: GO.BinaryNeq, X: fieldExpr, Y: y}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: cond}
		} else {
			ifs.Cond = cond
		}

		args[i] = `\"` + val + `\"`
	}

	retStmt := makeReturnStmtForError(g, field.Key+" must be equal to: "+strings.Join(args, " or "))

	ifs.Body.Add(retStmt)
	return ifs
}

func makeIfStmtForNotEquals(g *generator, r *analysis.Rule, field *analysis.StructField, fieldExpr GO.ExprNode) (ifs GO.IfStmt) {
	typ := field.Type
	for typ.Kind == analysis.TypeKindPtr {
		typ = *typ.Elem
	}

	args := make([]string, len(r.Args)) // for error message
	for i, a := range r.Args {
		val := a.Value

		var y GO.ExprNode
		if typ.Kind == analysis.TypeKindString {
			y = GO.StringLit(val)
		} else if a.Type == analysis.ArgTypeString {
			if typ.Kind.IsNumeric() && len(val) == 0 {
				y = GO.IntLit(0)
				val = "0"
			} else {
				y = GO.StringLit(val)
			}
		} else {
			y = GO.ValueLit(val)
		}

		cond := GO.BinaryExpr{Op: GO.BinaryEql, X: fieldExpr, Y: y}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLOr, X: ifs.Cond, Y: cond}
		} else {
			ifs.Cond = cond
		}

		args[i] = `\"` + val + `\"`
	}

	retStmt := makeReturnStmtForError(g, field.Key+" must not be equal to: "+strings.Join(args, " or "))

	ifs.Body.Add(retStmt)
	return ifs
}

func makeReturnStmtForError(g *generator, errmesg string) (ret GO.ReturnStmt) {
	errnew := GO.QualifiedIdent{"errors", "New"}
	ret.Result = GO.CallExpr{Fun: errnew, Args: GO.ArgsList{List: GO.StringLit(errmesg)}}
	return ret
}

func makeReturnStmtForErrorRaw(g *generator, errmesg string) (ret GO.ReturnStmt) {
	errnew := GO.QualifiedIdent{"errors", "New"}
	ret.Result = GO.CallExpr{Fun: errnew, Args: GO.ArgsList{List: GO.RawStringLit(errmesg)}}
	return ret
}

// addImport
func addImport(g *generator, path, name, local string) {
	// check that the package path hasn't yet been added to the imports
	for _, spec := range g.imports.Specs {
		if string(spec.Path) == path {
			return
		}
	}

	// if the local name is the same as the package name set it to empty
	if local == name {
		local = ""
	}

	spec := GO.ImportSpec{Path: GO.StringLit(path), Name: GO.Ident{local}}
	g.imports.Specs = append(g.imports.Specs, spec)
}

func sortImports(imports *GO.ImportDecl) {
	var specs1, specs2, specs3 []GO.ImportSpec
	for _, s := range imports.Specs {
		if strings.HasPrefix(string(s.Path), isvalidPkgPath) {
			specs3 = append(specs3, s)
		} else if i := strings.IndexByte(string(s.Path), '.'); i >= 0 {
			specs2 = append(specs2, s)
		} else {
			specs1 = append(specs1, s)
		}
	}

	var specs []GO.ImportSpec
	if len(specs1) > 0 {
		specs = append(specs, specs1...)
	}
	if len(specs2) > 0 {
		specs2[0].Doc = GO.NL{}
		specs = append(specs, specs2...)
	}
	if len(specs3) > 0 {
		specs3[0].Doc = GO.NL{}
		specs = append(specs, specs3...)
	}
	imports.Specs = specs
}
