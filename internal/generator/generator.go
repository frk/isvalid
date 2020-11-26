package generator

import (
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/frk/isvalid/internal/analysis"

	GO "github.com/frk/ast/golang"
)

var _ = log.Println

// TargetAnalysis holds the result of the target's analysis and is used
// as the input to the generator.
type TargetAnalysis struct {
	// the target struct
	ValidatorStruct *analysis.ValidatorStruct
	// additional info produced by the analysis
	Info *analysis.Info
}

// Generate produces the code for the given targets and writes it to w.
func Generate(w io.Writer, pkgName string, targets []*TargetAnalysis) error {
	file := new(file)
	for _, t := range targets {
		g := new(generator)
		g.vs = t.ValidatorStruct
		g.info = t.Info
		g.file = file

		buildValidateMethod(g)
	}

	// add an "init()" func if needed
	if len(file.init) > 0 {
		init := GO.FuncDecl{}
		init.Name.Name = "init"
		init.Body.List = file.init
		file.Decls = append([]GO.TopLevelDeclNode{init}, file.Decls...)
	}

	// final touch
	file.PkgName = pkgName
	file.Preamble = GO.LineComment{` DO NOT EDIT. This file was generated by "github.com/frk/isvalid".`}
	file.Imports = []GO.ImportDeclNode{newImportDecl(file)}

	// let's go
	return GO.Write(file.File, w)
}

// The impspec type holds info that is used to generate a GO.ImportSpec node.
type impspec struct {
	// The package path of the import spec.
	path string
	// The package name associated with the import spec.
	name string
	// If set, it indicates that the Name field should be set
	// in the generated GO.ImportSpec node.
	local bool
	// Number of package's with the same name. This value is used by those
	// packages to modify their name in order to not cause an import conflict.
	num int
}

// The file type holds info about the file for which the code is being generated.
type file struct {
	// The ast.
	GO.File
	// List of imports requried by the file.
	impset []*impspec
	// If set, it indicates that the file needs to import "errors".
	importErrors bool
	// If set, it indicates that the file needs to import "fmt".
	importFmt bool
	// List of statements to be produced for the body of an init function
	// at the top of the file. If the slice is empty then the init function
	// will not be generated.
	init []GO.StmtNode
}

// varcode holds a particular variable's type and rule information as well as
// partial AST nodes all needed by the generator to produce validation code for
// that variable. Here the term "variable" denotes struct fields and also the
// individual elements (and keys) of map/slice/array typed fields.
type varcode struct {
	// The type of the variable for which the code is being prepared.
	vtype analysis.Type
	// The expression of the variable for which the code is being prepared.
	vexpr GO.ExprNode
	// The field with which the variable is associated.
	field *analysis.StructField

	// If vtype is a map and the map-key has one or more rules associated
	// with it, the key varcode will be used for preparing the validation
	// code of vtype's keys.
	key *varcode
	// If vtype is a map, slice, or array and the type's element has one
	// or more rules associated with it, the elem varcode will be used for
	// preparing the validation code of vtype's elements.
	elem *varcode
	// If vtype is a struct, the fields varcode slice will be used for
	// preparing the validation code of the vtype's fields.
	fields []*varcode

	// The variable's set of rules. If the rules "requried" and/or "notnil"
	// were specified for the variable they will be removed from this slice
	// and assigned to their own fields as they require special attention.
	rules []*analysis.Rule
	// Set if the variable's original rule set includes the "required" rule, otherwise nil.
	required *analysis.Rule
	// Set if the variable's original rule set includes the "notnil" rule, otherwise nil.
	notnil *analysis.Rule

	// A list of if-statement AST nodes built from the rules slice and used
	// by the generator to produce code that will, according to those rules,
	// validate the variable.
	ruleifs []GO.IfStmt
	// An if-statement AST node built from the "required" rule and used by the
	// generator to produce code that checks the variable against the "zero" value.
	rqif *GO.IfStmt
	// An if-statement AST node built from the "notnil" rule and used by the
	// generator to produce code that checks the variable against the nil value.
	nnif *GO.IfStmt

	// A "nil guard" AST node built for pointer variables and used by the
	// generator to produce code that checks that the pointer is not nil.
	ng GO.ExprNode
	// If set it is used to declare a new variable from vexpr and wrap its
	// validation code inside a "sub block". This is built specifically to
	// avoid producing code with repeated pointer dereferencing and/or long
	// selector chains of variables that have a "complex" vexpr.
	sb []GO.StmtNode
}

// The generator type holds the state of the generator.
type generator struct {
	// The analyzed validator type for which the code is being generated.
	vs *analysis.ValidatorStruct
	// Additional info associated with the validator type's analysis.
	info *analysis.Info
	// The target file to be build by the generator.
	file *file
	// The generated Validation method's receiver
	recv GO.Ident
	// The main work of the generator.
	varcodes []*varcode
	// "before validate" hook code.
	beforeValidate GO.StmtNode
	// "after validate" hook code.
	afterValidate GO.StmtNode
}

// set of common nodes
var (
	ERR   = GO.Ident{"err"}
	NIL   = GO.Ident{"nil"}
	ERROR = GO.Ident{"error"}
)

// Builds the "Validate() error" method for the target validator struct.
func buildValidateMethod(g *generator) {
	g.recv = GO.Ident{"v"}
	buildHookCalls(g)
	buildVarCodes(g)

	body := assembleBody(g)
	method := GO.MethodDecl{}
	method.Recv.Name = g.recv
	method.Recv.Type = GO.Ident{g.vs.TypeName}
	method.Name.Name = "Validate"
	method.Type.Results = GO.ParamList{{Type: ERROR}}
	method.Body.List = body

	g.file.Decls = append(g.file.Decls, method)
}

// buildHookCalls builds AST nodes for hook method calls & error handling.
func buildHookCalls(g *generator) {
	if g.vs.BeforeValidate != nil {
		name := g.vs.BeforeValidate.Name
		call := GO.CallExpr{Fun: GO.QualifiedIdent{g.recv.Name, name}}
		assign := GO.AssignStmt{Token: GO.AssignDefine, Lhs: ERR, Rhs: call}
		binary := GO.BinaryExpr{Op: GO.BinaryNeq, X: ERR, Y: NIL}
		ifbody := GO.BlockStmt{[]GO.StmtNode{GO.ReturnStmt{ERR}}}
		g.beforeValidate = GO.IfStmt{Init: assign, Cond: binary, Body: ifbody}
	}

	if g.vs.AfterValidate != nil {
		name := g.vs.AfterValidate.Name
		call := GO.CallExpr{Fun: GO.QualifiedIdent{g.recv.Name, name}}
		assign := GO.AssignStmt{Token: GO.AssignDefine, Lhs: ERR, Rhs: call}
		binary := GO.BinaryExpr{Op: GO.BinaryNeq, X: ERR, Y: NIL}
		body := GO.BlockStmt{[]GO.StmtNode{GO.ReturnStmt{ERR}}}
		g.afterValidate = GO.IfStmt{Init: assign, Cond: binary, Body: body}
	}
}

// buildVarCodes builds the varcode for the generator's target validator struct.
func buildVarCodes(g *generator) {
	for _, f := range g.vs.Fields {
		if !f.ContainsRules() {
			continue
		}

		expr := GO.SelectorExpr{X: g.recv, Sel: GO.Ident{f.Name}}
		code := &varcode{vtype: f.Type, vexpr: expr, field: f}

		buildVarCode(g, code, f.RuleTag)
		g.varcodes = append(g.varcodes, code)
	}
}

// buildVarCode builds individual nodes of the AST for the given varcode.
func buildVarCode(g *generator, code *varcode, tn *analysis.TagNode) {
	// split off the "required" and "notnil" rules, they need special attention.
	for _, r := range tn.Rules {
		if r.Name == "required" {
			code.required = r
		} else if r.Name == "notnil" {
			code.notnil = r
		} else {
			code.rules = append(code.rules, r)
		}
	}

	buildVarCodeNilGuard(g, code)
	buildVarCodeRequired(g, code)
	buildVarCodeNotnil(g, code)
	buildVarCodeSubBlock(g, code)
	buildVarCodeRules(g, code)

	switch code.vtype.Kind {
	case analysis.TypeKindSlice, analysis.TypeKindArray:
		if tn.Elem != nil {
			expr := GO.Ident{"e"}
			elem := &varcode{vtype: *code.vtype.Elem, vexpr: expr, field: code.field}
			buildVarCode(g, elem, tn.Elem)
			code.elem = elem
		}
	case analysis.TypeKindMap:
		if tn.Key != nil {
			expr := GO.Ident{"k"}
			key := &varcode{vtype: *code.vtype.Key, vexpr: expr, field: code.field}
			buildVarCode(g, key, tn.Key)
			code.key = key
		}
		if tn.Elem != nil {
			expr := GO.Ident{"e"}
			elem := &varcode{vtype: *code.vtype.Elem, vexpr: expr, field: code.field}
			buildVarCode(g, elem, tn.Elem)
			code.elem = elem
		}
	case analysis.TypeKindStruct:
		for _, f := range code.vtype.Fields {
			if !f.ContainsRules() {
				continue
			}

			expr := GO.SelectorExpr{X: code.vexpr, Sel: GO.Ident{f.Name}}
			next := &varcode{vtype: f.Type, vexpr: expr, field: f}

			buildVarCode(g, next, f.RuleTag)
			code.fields = append(code.fields, next)

		}
	}
}

// buildVarCodeNilGuard builds the "nil guard" AST node for the given varcode.
func buildVarCodeNilGuard(g *generator, code *varcode) {
	if code.vtype.Kind != analysis.TypeKindPtr {
		return // nothing to do
	}

	// logical op, and equality op
	lop, eop := GO.BinaryLAnd, GO.BinaryNeq
	if code.required != nil || code.notnil != nil {
		lop, eop = GO.BinaryLOr, GO.BinaryEql
	}

	cond := GO.BinaryExpr{Op: eop, X: code.vexpr, Y: NIL}
	code.vexpr = GO.PointerIndirectionExpr{code.vexpr}
	code.vtype = *code.vtype.Elem

	// handle multiple pointers
	for code.vtype.Kind == analysis.TypeKindPtr {
		binx := GO.BinaryExpr{Op: eop, X: code.vexpr, Y: NIL}

		cond = GO.BinaryExpr{Op: lop, X: cond, Y: binx}
		code.vexpr = GO.PointerIndirectionExpr{code.vexpr}
		code.vtype = *code.vtype.Elem
	}

	code.ng = cond
}

// buildVarCodeRequired builds the IfStmt AST node for the varcode's "required" rule.
func buildVarCodeRequired(g *generator, code *varcode) {
	if code.required == nil {
		return // nothing to do
	}

	cond := newRequiredExpr(g, code)
	if cond != nil && code.ng != nil {
		cond = GO.BinaryExpr{Op: GO.BinaryLOr, X: code.ng, Y: cond}
	} else if code.ng != nil {
		cond = code.ng
	}

	if len(code.required.Context) > 0 {
		opt := GO.SelectorExpr{X: g.recv, Sel: GO.Ident{g.vs.ContextOption.Name}}
		bin := GO.BinaryExpr{Op: GO.BinaryEql, X: opt, Y: GO.StringLit(code.required.Context)}
		cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: cond, Y: bin}
	}

	code.rqif = &GO.IfStmt{Cond: cond}
	code.rqif.Body.Add(newErrorReturnStmt(g, code, code.required))
}

// buildVarCodeNotnil builds the IfStmt AST node for the varcode's "notnil" rule.
func buildVarCodeNotnil(g *generator, code *varcode) {
	if code.notnil == nil {
		return // nothing to do
	}

	cond := newNotnilExpr(g, code)
	if cond != nil && code.ng != nil {
		cond = GO.BinaryExpr{Op: GO.BinaryLOr, X: code.ng, Y: cond}
	} else if code.ng != nil {
		cond = code.ng
	}

	if len(code.notnil.Context) > 0 {
		opt := GO.SelectorExpr{X: g.recv, Sel: GO.Ident{g.vs.ContextOption.Name}}
		bin := GO.BinaryExpr{Op: GO.BinaryEql, X: opt, Y: GO.StringLit(code.notnil.Context)}
		cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: cond, Y: bin}
	}

	code.nnif = &GO.IfStmt{Cond: cond}
	code.nnif.Body.Add(newErrorReturnStmt(g, code, code.notnil))
}

// buildVarCodeSubBlock builds the "sub block" AST node for the varcode.
func buildVarCodeSubBlock(g *generator, code *varcode) {
	// no nil-guard = no sub-block necessary
	if code.ng == nil {
		// MAYBE-TODO(mkopriva): it might be worth building a sub-block
		// for struct variables with deeply nested fields to avoid producing
		// code that references the "leaf" fields via a long chained selector.
		return // nothing to do
	}

	// no subfields and there's a "notnil" or "required" rule then we do not
	// need a sub-block, instead we want to chain the "notnil"/"required" IfStmt
	// with the IfStmt produced from ruleifs.
	if len(code.vtype.Fields) == 0 && (code.nnif != nil || code.rqif != nil) {
		return // nothing to do
	}

	// no subfields and there's at most 1 rule, no sub-block
	if len(code.vtype.Fields) == 0 && len(code.rules) < 2 {
		return // nothing to do
	}

	v := GO.Ident{"f"}
	a := GO.AssignStmt{Token: GO.AssignDefine, Lhs: v, Rhs: code.vexpr}

	code.vexpr = v
	code.sb = append(code.sb, a)
}

// buildVarCodeRules builds IfStmt AST nodes for the varcode's rules.
func buildVarCodeRules(g *generator, code *varcode) {
	for _, r := range code.rules {
		ifs := newRuleIfStmt(g, code, r)
		if len(r.Context) > 0 {
			opt := GO.SelectorExpr{X: g.recv, Sel: GO.Ident{g.vs.ContextOption.Name}}
			bin := GO.BinaryExpr{Op: GO.BinaryEql, X: opt, Y: GO.StringLit(r.Context)}
			ifs.Cond = GO.ParenExpr{GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: bin}}
		}

		code.ruleifs = append(code.ruleifs, ifs)
	}
}

// assembleBody assembles the built AST nodes into a set of statements that represent
// the body of the "Validate() error" method.
func assembleBody(g *generator) (body []GO.StmtNode) {
	if g.beforeValidate != nil {
		body = append(body, g.beforeValidate)
	}
	for _, code := range g.varcodes {
		if s := assembleVarCode(g, code); s != nil {
			body = append(body, s)
		}
	}
	if g.afterValidate != nil {
		body = append(body, g.afterValidate)
	}

	// the closing return statement
	retStmt := GO.ReturnStmt{Result: NIL}
	if g.vs.ErrorHandler != nil && g.vs.ErrorHandler.IsAggregator {
		eh := GO.SelectorExpr{X: g.recv, Sel: GO.Ident{g.vs.ErrorHandler.Name}}
		retStmt.Result = GO.CallExpr{Fun: GO.SelectorExpr{X: eh, Sel: GO.Ident{"Out"}}}
	}
	return append(body, retStmt)
}

// assembleVarCode assembles the varcode's AST parts into a single statement node.
func assembleVarCode(g *generator, code *varcode) GO.StmtNode {
	// block for subfields
	var stmtlist GO.StmtList
	for _, code := range code.fields {
		if s := assembleVarCode(g, code); s != nil {
			stmtlist = append(stmtlist, s)
		}
	}
	if len(stmtlist) > 0 {
		return assembleVarCodeSubBlock(g, code, stmtlist)
	}

	// ifstmt for the varcode's rules
	ifs := assembleVarCodeRules(g, code)
	if ifs.Cond != nil {
		return assembleVarCodeSubBlock(g, code, ifs)
	}

	if code.key != nil || code.elem != nil {
		node := assembleVarCodeKeyElem(g, code)
		if node.Clause != nil {
			if code.ng != nil {
				return GO.IfStmt{Cond: code.ng, Body: GO.BlockStmt{[]GO.StmtNode{node}}}
			}
			return assembleVarCodeSubBlock(g, code, node)
		}
	}
	return nil
}

// assembleVarCodeSubBlock assembles the varcode's "sub block" with the given stmt as its body.
func assembleVarCodeSubBlock(g *generator, code *varcode, stmt GO.StmtNode) GO.StmtNode {
	if code.sb == nil {
		return stmt
	}

	bs := GO.BlockStmt{append(code.sb, stmt)}
	if code.rqif != nil {
		ifs := *code.rqif
		ifs.Else = bs
		return ifs
	} else if code.nnif != nil {
		ifs := *code.nnif
		ifs.Else = bs
		return ifs
	} else if code.ng != nil {
		return GO.IfStmt{Cond: code.ng, Body: bs}
	}
	return bs
}

// assembleVarCodeRules assembles the varcode rules' ASTs into a single if-else chain.
func assembleVarCodeRules(g *generator, code *varcode) GO.IfStmt {
	var root GO.IfStmt

	// merge "required" or "notnil" IfStmt with the ruleifs slice
	var iflist []GO.IfStmt
	if code.rqif != nil {
		iflist = []GO.IfStmt{*code.rqif}
	} else if code.nnif != nil {
		iflist = []GO.IfStmt{*code.nnif}
	}
	iflist = append(iflist, code.ruleifs...)

	// chain the if-statements
	for i := len(iflist) - 1; i >= 0; i-- {
		ifs := iflist[i]
		if root.Cond == nil {
			root = ifs
		} else {
			ifs.Else = root
			root = ifs
		}
	}

	// if "nilguard" present but no "required" and no "notnil" then if we have
	// only a single rule we can merge its conditional with that of the "nilguard",
	// note that this works only with single rules, multiple rules would end up
	// in else-ifs without the nilguard and could cause panic.
	if (code.ng != nil && code.rqif == nil && code.nnif == nil) && len(code.ruleifs) == 1 {
		root.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: code.ng, Y: root.Cond}
	}

	return root
}

// assembleVarCodeKeyElem assembles the key/elem varcode and returns the result in a ForStmt node.
func assembleVarCodeKeyElem(g *generator, code *varcode) (fs GO.ForStmt) {
	rc := GO.ForRangeClause{X: code.vexpr, Define: true}
	switch code.vtype.Kind {
	case analysis.TypeKindSlice, analysis.TypeKindArray:
		rc.Key = GO.Ident{"_"}
		rc.Value = GO.Ident{"e"}
	case analysis.TypeKindMap:
		rc.Key = GO.Ident{"k"}
		rc.Value = GO.Ident{"e"}
	default:
		panic("shouldn't reach")
	}
	fs.Clause = rc

	if code.vtype.Kind == analysis.TypeKindMap {
		if sn := assembleVarCode(g, code.key); sn != nil {
			fs.Body.List = append(fs.Body.List, sn)
		}
	}
	if sn := assembleVarCode(g, code.elem); sn != nil {
		fs.Body.List = append(fs.Body.List, sn)
	}
	return fs
}

// newRequiredExpr produces an expression that checks the varcode's variable against the "zero" value.
func newRequiredExpr(g *generator, code *varcode) GO.ExprNode {
	switch code.vtype.Kind {
	case analysis.TypeKindString, analysis.TypeKindMap, analysis.TypeKindSlice:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: GO.CallLenExpr{code.vexpr}, Y: GO.IntLit(0)}
	case analysis.TypeKindInt, analysis.TypeKindInt8, analysis.TypeKindInt16, analysis.TypeKindInt32, analysis.TypeKindInt64:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: code.vexpr, Y: GO.IntLit(0)}
	case analysis.TypeKindUint, analysis.TypeKindUint8, analysis.TypeKindUint16, analysis.TypeKindUint32, analysis.TypeKindUint64:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: code.vexpr, Y: GO.IntLit(0)}
	case analysis.TypeKindFloat32, analysis.TypeKindFloat64:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: code.vexpr, Y: GO.ValueLit("0.0")}
	case analysis.TypeKindBool:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: code.vexpr, Y: GO.ValueLit("false")}
	case analysis.TypeKindPtr, analysis.TypeKindInterface:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: code.vexpr, Y: NIL}
	}
	return nil
}

// newNotnilExpr produces an expression that checks the varcode's variable against the nil value.
func newNotnilExpr(g *generator, code *varcode) GO.ExprNode {
	switch code.vtype.Kind {
	case analysis.TypeKindPtr, analysis.TypeKindSlice, analysis.TypeKindMap, analysis.TypeKindInterface:
		return GO.BinaryExpr{Op: GO.BinaryEql, X: code.vexpr, Y: NIL}
	}
	return nil
}

// newRuleIfStmt produces an if-statement that checks the varcode's variable against the given rule.
func newRuleIfStmt(g *generator, code *varcode, r *analysis.Rule) (ifs GO.IfStmt) {
	rt := g.info.RuleTypeMap[r.Name]
	switch rx := rt.(type) {
	// TODO deal with RuleTypeNop?
	case analysis.RuleTypeIsValid:
		return newRuleTypeIsValidIfStmt(g, code, r)
	case analysis.RuleTypeEnum:
		return newRuleTypeEnumIfStmt(g, code, r)
	case analysis.RuleTypeBasic:
		if r.Name == "len" {
			return newRuleTypeBasicLenIfStmt(g, code, r)
		} else if r.Name == "rng" {
			return newRuleTypeBasicRngIfStmt(g, code, r)
		}
		return newRuleTypeBasicIfStmt(g, code, r)
	case analysis.RuleTypeFunc:
		if rx.LOp > 0 {
			return newRuleTypeFuncChainIfStmt(g, code, r, rx)
		}
		return newRuleTypeFuncIfStmt(g, code, r, rx)
	}

	panic("shouldn't reach")
	return ifs
}

// newRuleTypeIsValidIfStmt produces an if-statement that checks the varcode's variable using the "IsValid()" method.
func newRuleTypeIsValidIfStmt(g *generator, code *varcode, r *analysis.Rule) (ifs GO.IfStmt) {
	x := code.vexpr
	if code.field.Type.Kind == analysis.TypeKindPtr {
		x = GO.ParenExpr{x}
	}
	sel := GO.SelectorExpr{X: x, Sel: GO.Ident{"IsValid"}}
	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: GO.CallExpr{Fun: sel}}
	ifs.Body.Add(newErrorReturnStmt(g, code, r))
	return ifs
}

// newRuleTypeEnumIfStmt produces an if-statement that checks the varcode's variable against a set of enums.
func newRuleTypeEnumIfStmt(g *generator, code *varcode, r *analysis.Rule) (ifs GO.IfStmt) {
	typ := code.field.Type.PtrBase()
	ident := typ.PkgPath + "." + typ.Name
	enums := g.info.EnumMap[ident]

	for _, e := range enums {
		id := GO.ExprNode(GO.Ident{e.Name})
		if g.info.PkgPath != e.PkgPath {
			imp := addimport(g.file, e.PkgPath)
			id = GO.QualifiedIdent{imp.name, e.Name}
		}

		cond := GO.BinaryExpr{Op: GO.BinaryNeq, X: code.vexpr, Y: id}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: cond}
		} else {
			ifs.Cond = cond
		}
	}

	ifs.Body.Add(newErrorReturnStmt(g, code, r))
	return ifs
}

// newRuleTypeBasicIfStmt produces an if-statement that checks the varcode's variable using basic comparison operators.
func newRuleTypeBasicIfStmt(g *generator, code *varcode, r *analysis.Rule) (ifs GO.IfStmt) {
	typ := code.field.Type
	for typ.Kind == analysis.TypeKindPtr {
		typ = *typ.Elem
	}

	binop := basicRuleToBinaryOp[r.Name]
	logop := basicRuleToLogicalOp[r.Name]

	for _, a := range r.Args {
		cond := GO.BinaryExpr{Op: binop, X: code.vexpr, Y: newArgValueExpr(g, r, a, typ)}
		if ifs.Cond != nil {
			ifs.Cond = GO.BinaryExpr{Op: logop, X: ifs.Cond, Y: cond}
		} else {
			ifs.Cond = cond
		}
	}

	ifs.Body.Add(newErrorReturnStmt(g, code, r))
	return ifs
}

// newRuleTypeBasicLenIfStmt produces an if-statement that checks the varcode variable's length.
func newRuleTypeBasicLenIfStmt(g *generator, code *varcode, r *analysis.Rule) (ifs GO.IfStmt) {
	typ := analysis.Type{Kind: analysis.TypeKindInt} // len(T) returns an int

	if len(r.Args) == 1 {
		a := r.Args[0]
		ifs.Cond = GO.BinaryExpr{Op: GO.BinaryNeq, X: GO.CallLenExpr{code.vexpr}, Y: newArgValueExpr(g, r, a, typ)}
		ifs.Body.Add(newErrorReturnStmt(g, code, r))
	} else { // len(r.Args) == 2 is assumed
		a1, a2 := r.Args[0], r.Args[1]
		if len(a1.Value) > 0 && len(a2.Value) == 0 {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLss, X: GO.CallLenExpr{code.vexpr}, Y: newArgValueExpr(g, r, a1, typ)}
			ifs.Body.Add(newErrorReturnStmt(g, code, r))
		} else if len(a1.Value) == 0 && len(a2.Value) > 0 {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryGtr, X: GO.CallLenExpr{code.vexpr}, Y: newArgValueExpr(g, r, a2, typ)}
			ifs.Body.Add(newErrorReturnStmt(g, code, r))
		} else {
			ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLOr,
				X: GO.BinaryExpr{Op: GO.BinaryLss, X: GO.CallLenExpr{code.vexpr}, Y: newArgValueExpr(g, r, a1, typ)},
				Y: GO.BinaryExpr{Op: GO.BinaryGtr, X: GO.CallLenExpr{code.vexpr}, Y: newArgValueExpr(g, r, a2, typ)}}
			ifs.Cond = GO.ParenExpr{ifs.Cond}
			ifs.Body.Add(newErrorReturnStmt(g, code, r))
		}
	}
	return ifs
}

// newRuleTypeBasicRngIfStmt produces an if-statement that checks the varcode variable's numeric range.
func newRuleTypeBasicRngIfStmt(g *generator, code *varcode, r *analysis.Rule) (ifs GO.IfStmt) {
	a1, a2 := r.Args[0], r.Args[1]

	ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLOr,
		X: GO.BinaryExpr{Op: GO.BinaryLss, X: code.vexpr, Y: newArgValueExpr(g, r, a1, code.field.Type.PtrBase())},
		Y: GO.BinaryExpr{Op: GO.BinaryGtr, X: code.vexpr, Y: newArgValueExpr(g, r, a2, code.field.Type.PtrBase())}}
	ifs.Cond = GO.ParenExpr{ifs.Cond}
	ifs.Body.Add(newErrorReturnStmt(g, code, r))
	return ifs
}

// newRuleTypeFuncIfStmt produces an if-statement that checks the varcode's variable using the rule's function.
func newRuleTypeFuncIfStmt(g *generator, code *varcode, r *analysis.Rule, rt analysis.RuleTypeFunc) (ifs GO.IfStmt) {
	imp := addimport(g.file, rt.PkgPath)
	retStmt := newErrorReturnStmt(g, code, r)

	fn := GO.QualifiedIdent{imp.name, rt.FuncName}
	call := GO.CallExpr{Fun: fn, Args: GO.ArgsList{List: code.vexpr}}
	args := GO.ExprList{code.vexpr}

	argtypes := rt.TypesForArgs(r.Args)
	for i, a := range r.Args {
		args = append(args, newArgValueExpr(g, r, a, argtypes[i]))

		if r.Name == "re" {
			// if this is the regexp rule, then add a registry
			// call for the init function and make sure to use
			// raw string literals.
			regrx := GO.CallExpr{Fun: GO.QualifiedIdent{imp.name, "RegisterRegexp"}}
			regrx.Args.List = GO.RawStringLit(a.Value)
			g.file.init = append(g.file.init, GO.ExprStmt{regrx})
		}
	}
	call.Args.List = args
	ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
	ifs.Body.Add(retStmt)
	return ifs
}

// newRuleTypeFuncChainIfStmt produces an if-statement that checks the varcode's variable
// using the rule's function invoking it in a chain for each of the rule's arguments.
func newRuleTypeFuncChainIfStmt(g *generator, code *varcode, r *analysis.Rule, rt analysis.RuleTypeFunc) (ifs GO.IfStmt) {
	imp := addimport(g.file, rt.PkgPath)
	retStmt := newErrorReturnStmt(g, code, r)

	argtypes := rt.TypesForArgs(r.Args)
	for i, a := range r.Args {
		call := GO.CallExpr{Fun: GO.QualifiedIdent{imp.name, rt.FuncName}}
		call.Args.List = GO.ExprList{code.vexpr, newArgValueExpr(g, r, a, argtypes[i])}

		switch rt.LOp {
		case analysis.LogicalNot: // x || x || x....
			if ifs.Cond != nil {
				ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLOr, X: ifs.Cond, Y: call}
			} else {
				ifs.Cond = call
			}
		case analysis.LogicalAnd: // !x || !x || !x....
			if ifs.Cond != nil {
				ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLOr, X: ifs.Cond, Y: GO.UnaryExpr{Op: GO.UnaryNot, X: call}}
			} else {
				ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
			}
		case analysis.LogicalOr: // !x && !x && !x....
			if ifs.Cond != nil {
				ifs.Cond = GO.BinaryExpr{Op: GO.BinaryLAnd, X: ifs.Cond, Y: GO.UnaryExpr{Op: GO.UnaryNot, X: call}}
			} else {
				ifs.Cond = GO.UnaryExpr{Op: GO.UnaryNot, X: call}
			}
		}
	}
	ifs.Body.Add(retStmt)
	return ifs
}

// newArgValueExpr produces an expression of the given argument's value.
func newArgValueExpr(g *generator, r *analysis.Rule, a *analysis.RuleArg, t analysis.Type) GO.ExprNode {
	if a.Type == analysis.ArgTypeField {
		return newArgFieldSelectorExpr(g, r, a, t)
	}
	return newArgConstExpr(g, r, a, t)
}

// newArgFieldSelectorExpr produces a field selector expression of the given argument's value.
func newArgFieldSelectorExpr(g *generator, r *analysis.Rule, a *analysis.RuleArg, t analysis.Type) (x GO.ExprNode) {
	var selector = g.info.SelectorMap[a.Value]
	var last *analysis.StructField

	x = g.recv
	for _, f := range selector {
		x = GO.SelectorExpr{X: x, Sel: GO.Ident{f.Name}}
		last = f
	}

	if t.NeedsConversion(last.Type) {
		cx := GO.CallExpr{}
		cx.Fun = GO.Ident{last.Type.String()}
		cx.Args = GO.ArgsList{List: x}
		x = cx
	}
	return x
}

// newArgConstExpr produces a constant expression of the given argument's value.
func newArgConstExpr(g *generator, r *analysis.Rule, a *analysis.RuleArg, t analysis.Type) (x GO.ExprNode) {
	var userawstring bool
	rt := g.info.RuleTypeMap[r.Name]
	if fn, ok := rt.(analysis.RuleTypeFunc); ok {
		userawstring = fn.UseRawString
	}

	if t.IsEmptyInterface {
		if a.Type == analysis.ArgTypeString {
			if userawstring {
				return GO.RawStringLit(a.Value)
			}
			return GO.ValueLit(strconv.Quote(a.Value))
		}

		return GO.ValueLit(a.Value)
	}

	if t.Kind == analysis.TypeKindString {
		if userawstring {
			return GO.RawStringLit(a.Value)
		}
		return GO.ValueLit(strconv.Quote(a.Value))
	}

	switch a.Type {
	case analysis.ArgTypeUnknown:
		switch t.Kind {
		case analysis.TypeKindString:
			x = GO.StringLit("")
		case analysis.TypeKindInt, analysis.TypeKindInt8, analysis.TypeKindInt16, analysis.TypeKindInt32, analysis.TypeKindInt64:
			x = GO.IntLit(0)
		case analysis.TypeKindUint, analysis.TypeKindUint8, analysis.TypeKindUint16, analysis.TypeKindUint32, analysis.TypeKindUint64:
			x = GO.IntLit(0)
		case analysis.TypeKindFloat32, analysis.TypeKindFloat64:
			x = GO.ValueLit("0.0")
		case analysis.TypeKindBool:
			x = GO.ValueLit("false")
		case analysis.TypeKindPtr, analysis.TypeKindInterface, analysis.TypeKindMap, analysis.TypeKindSlice:
			x = NIL
		}
		return x

	case analysis.ArgTypeBool:
		x = GO.ValueLit(a.Value)
		// TODO
		return x

	case analysis.ArgTypeInt:
		x = GO.ValueLit(a.Value)
		// TODO
		return x

	case analysis.ArgTypeFloat:
		x = GO.ValueLit(a.Value)
		// TODO
		return x

	case analysis.ArgTypeString:
		if userawstring {
			x = GO.RawStringLit(a.Value)
		} else {
			x = GO.ValueLit(strconv.Quote(a.Value))
		}
		// TODO
		return x

	case analysis.ArgTypeField:
		x = g.recv
		for _, f := range g.info.SelectorMap[a.Value] {
			x = GO.SelectorExpr{X: x, Sel: GO.Ident{f.Name}}
		}
		return x
	}

	panic("shouldn't reach")
	return nil
}

// newErrorReturnStmt produces statement node that returns an error value.
func newErrorReturnStmt(g *generator, code *varcode, r *analysis.Rule) GO.StmtNode {
	// Build code for custom handler, if one exists.
	if g.vs.ErrorHandler != nil {
		args := make(GO.ExprList, 3)
		args[0] = GO.StringLit(code.field.Key)
		args[1] = code.vexpr
		args[2] = GO.StringLit(r.Name)

		for _, a := range r.Args {
			switch a.Type {
			case analysis.ArgTypeField:
				x := GO.ExprNode(g.recv)
				for _, f := range g.info.SelectorMap[a.Value] {
					x = GO.SelectorExpr{X: x, Sel: GO.Ident{f.Name}}
				}
				args = append(args, x)
			case analysis.ArgTypeString:
				args = append(args, GO.StringLit(a.Value))
			case analysis.ArgTypeUnknown:
				args = append(args, GO.StringLit(""))
			default:
				args = append(args, GO.ValueLit(a.Value))
			}
		}

		eh := GO.SelectorExpr{X: GO.QualifiedIdent{"v", g.vs.ErrorHandler.Name}, Sel: GO.Ident{"Error"}}
		call := GO.CallExpr{Fun: eh, Args: GO.ArgsList{List: args}}
		if g.vs.ErrorHandler.IsAggregator {
			return GO.ExprStmt{call}
		} else {
			return GO.ReturnStmt{Result: call}
		}
	}

	// If no custom handler exists, then return the default error message.
	return GO.ReturnStmt{newErrorExpr(g, code, r)}

}

// newErrorExpr produces an error value expression.
func newErrorExpr(g *generator, code *varcode, r *analysis.Rule) (errExpr GO.ExprNode) {
	rt := g.info.RuleTypeMap[r.Name]
	switch rt.(type) {
	case analysis.RuleTypeIsValid, analysis.RuleTypeEnum, analysis.RuleTypeFunc:
		if f, ok := rt.(analysis.RuleTypeFunc); !ok || f.IsCustom {
			text := code.field.Key + " is not valid" // default error text for custom specs

			errText := GO.ValueLit(strconv.Quote(text))
			g.file.importErrors = true
			errExpr = GO.CallExpr{Fun: GO.QualifiedIdent{"errors", "New"},
				Args: GO.ArgsList{List: errText}}

			return errExpr
		}
	}

	// Resolve the alternative form of the error message, currently only "len" needs this.
	var altform int
	if r.Name == "len" && len(r.Args) == 2 {
		if len(r.Args[0].Value) > 0 && len(r.Args[1].Value) == 0 {
			altform = 1
		} else if len(r.Args[0].Value) == 0 && len(r.Args[1].Value) > 0 {
			altform = 2
		} else {
			altform = 3
		}
	}

	// Get the error config.
	conf := errorConfigMap[r.Name][altform]
	text := code.field.Key + " " + conf.text // primary error text
	typ := code.field.Type.PtrBase()

	var refs GO.ExprList
	if !conf.omitArgs {
		var args []string
		for _, a := range r.Args {
			// if the field's type is numeric an unknown arg can be overwritten as 0.
			if a.Type == analysis.ArgTypeUnknown && typ.Kind.IsNumeric() {
				a = &analysis.RuleArg{Type: analysis.ArgTypeInt, Value: "0"}
			}

			// skip empty
			if len(a.Value) == 0 {
				continue
			}

			if a.Type == analysis.ArgTypeField {
				x := GO.ExprNode(g.recv)
				for _, f := range g.info.SelectorMap[a.Value] {
					x = GO.SelectorExpr{X: x, Sel: GO.Ident{f.Name}}
				}
				refs = append(refs, x)
				args = append(args, "%v")
			} else if a.Type == analysis.ArgTypeString {
				args = append(args, strconv.Quote(a.Value))
			} else {
				args = append(args, a.Value)
			}
		}

		if len(args) > 0 {
			text += ": " + strings.Join(args, conf.argSep)
		}
	}

	if len(conf.suffix) > 0 {
		text += " " + conf.suffix
	}

	errText := GO.ValueLit(strconv.Quote(text))
	if len(refs) > 0 {
		g.file.importFmt = true
		errExpr = GO.CallExpr{Fun: GO.QualifiedIdent{"fmt", "Errorf"},
			Args: GO.ArgsList{List: append(GO.ExprList{errText}, refs...)}}
	} else {
		g.file.importErrors = true
		errExpr = GO.CallExpr{Fun: GO.QualifiedIdent{"errors", "New"},
			Args: GO.ArgsList{List: errText}}
	}
	return errExpr
}

// newImportDecl produces an import declaration for packages that are to be imported by the given file.
func newImportDecl(f *file) *GO.ImportDecl {
	imports := new(GO.ImportDecl)
	if f.importErrors {
		imports.Specs = append(imports.Specs, GO.ImportSpec{Path: "errors"})
	}
	if f.importFmt {
		imports.Specs = append(imports.Specs, GO.ImportSpec{Path: "fmt"})
	}

	for _, imp := range f.impset {
		spec := GO.ImportSpec{Path: GO.StringLit(imp.path)}
		if imp.local {
			spec.Name.Name = imp.name
		}
		imports.Specs = append(imports.Specs, spec)
	}

	// group the imports into 3 groups separated by a new line, the 1st group
	// will contain imports from the standard library, the 3rd group will contain
	// imports from github.com/frk/isvalid..., and the 2nd group will contain
	// the rest of the imports.
	var specs1, specs2, specs3 []GO.ImportSpec
	for _, s := range imports.Specs {
		if strings.HasPrefix(string(s.Path), `github.com/frk/isvalid`) {
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

	return imports
}

// addimport adds a new impspec to the file's impset if it is not already a member of the set.
func addimport(f *file, path string) *impspec {
	name := path
	if i := strings.LastIndexByte(name, '/'); i > -1 {
		name = name[i+1:]
	}

	var namesake *impspec
	for _, imp := range f.impset {
		// already added, exit
		if imp.path == path {
			return imp
		}

		// retain import that has the same name
		if imp.name == name {
			namesake = imp
		}
	}

	imp := &impspec{path: path, name: name}
	if namesake != nil {
		namesake.num += 1
		imp.name = name + strconv.Itoa(namesake.num)
		imp.local = true
	}

	f.impset = append(f.impset, imp)
	return imp
}

var basicRuleToBinaryOp = map[string]GO.BinaryOp{
	"eq":  GO.BinaryNeq,
	"ne":  GO.BinaryEql,
	"gt":  GO.BinaryLeq,
	"lt":  GO.BinaryGeq,
	"gte": GO.BinaryLss,
	"lte": GO.BinaryGtr,
	"min": GO.BinaryLss,
	"max": GO.BinaryGtr,
}

var basicRuleToLogicalOp = map[string]GO.BinaryOp{
	"eq": GO.BinaryLAnd,
	"ne": GO.BinaryLOr,
}

// error message configuation
type errorConfig struct {
	// primary text of the error message
	text string
	// if set, append the suffix to the error message
	suffix string
	// if the validation rule takes multiple arguments, separate them with
	// argSep in the errro message
	argSep string
	// even if the validation rule takes an argument, do not display it
	// in the error message
	omitArgs bool
}

// A map of errorConfigs used for generating error messages. The first key is
// the rule's name and the second key maps the alternative forms of the error.
//
// MAYBE-TODO(mkopriva): this doesn't feel right, a nicer solution would be welcome...
var errorConfigMap = map[string]map[int]errorConfig{
	"required": {0: {text: "is required"}},
	"notnil":   {0: {text: "cannot be nil"}},
	"email":    {0: {text: "must be a valid email"}},
	"url":      {0: {text: "must be a valid URL"}},
	"uri":      {0: {text: "must be a valid URI"}},
	"pan":      {0: {text: "must be a valid PAN"}},
	"cvv":      {0: {text: "must be a valid CVV"}},
	"ssn":      {0: {text: "must be a valid SSN"}},
	"ein":      {0: {text: "must be a valid EIN"}},
	"numeric":  {0: {text: "must contain only digits [0-9]"}},
	"hex":      {0: {text: "must be a valid hexadecimal string"}},
	"hexcolor": {0: {text: "must be a valid hex color code"}},
	"alphanum": {0: {text: "must be an alphanumeric string"}},
	"cidr":     {0: {text: "must be a valid CIDR"}},
	"phone":    {0: {text: "must be a valid phone number", omitArgs: true}},
	"zip":      {0: {text: "must be a valid zip code", omitArgs: true}},
	"uuid":     {0: {text: "must be a valid UUID", omitArgs: true}},
	"ip":       {0: {text: "must be a valid IP", omitArgs: true}},
	"mac":      {0: {text: "must be a valid MAC", omitArgs: true}},
	"iso":      {0: {text: "must be a valid ISO"}},
	"rfc":      {0: {text: "must be a valid RFC"}},
	"re":       {0: {text: "must match the regular expression"}},
	"prefix":   {0: {text: "must be prefixed with", argSep: " or "}},
	"suffix":   {0: {text: "must be suffixed with", argSep: " or "}},
	"contains": {0: {text: "must contain substring", argSep: " or "}},
	"eq":       {0: {text: "must be equal to", argSep: " or "}},
	"ne":       {0: {text: "must not be equal to", argSep: " or "}},
	"gt":       {0: {text: "must be greater than"}},
	"lt":       {0: {text: "must be less than"}},
	"gte":      {0: {text: "must be greater than or equal to"}},
	"lte":      {0: {text: "must be less than or equal to"}},
	"min":      {0: {text: "must be greater than or equal to"}},
	"max":      {0: {text: "must be less than or equal to"}},
	"rng":      {0: {text: "must be between", argSep: " and "}},
	"len": {
		0: {text: "must be of length"},
		1: {text: "must be of length at least"},
		2: {text: "must be of length at most"},
		3: {text: "must be of length between", argSep: " and ", suffix: "(inclusive)"},
	},
}
