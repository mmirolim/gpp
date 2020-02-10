package macro

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"log"
	"strings"

	"github.com/kr/pretty"
	"golang.org/x/tools/go/ast/astutil"
)

const (
	MacroSymbol     = "_μ"
	Seq_μTypeSymbol = "seq_μ"
	Try_μSymbol     = "Try_μ"
	Log_μSymbol     = "Log_μ"
	MacroPkgPath    = "github.com/mmirolim/gpp/macro"
)

// TODO move to context?
var ApplyState = struct {
	IsOuterMacro bool
	File         *ast.File
	Fset         *token.FileSet
}{}

// define custom macro expand functions
// TODO make settable, prefixed by modulename?
var MacroExpanders = map[string]MacroExpander{
	Seq_μTypeSymbol: MacroNewSeq,
	Try_μSymbol:     MacroTryExpand,
	Log_μSymbol:     MacroLogExpand,
	// TODO change to package path
	"macro." + Seq_μTypeSymbol: MacroNewSeq,
	"macro." + Try_μSymbol:     MacroTryExpand,
	"macro." + Log_μSymbol:     MacroLogExpand,
}

var MacroDecl = map[string]*ast.FuncDecl{}

type MacroExpander func(cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc,
) bool

// TODO should resolve across packages
func AllMacroMethods(f *ast.File) map[string]*ast.FuncDecl {
	methods := make(map[string]*ast.FuncDecl)
	for _, decl := range f.Decls {
		fnDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fnDecl.Recv == nil {
			continue
		}
		typeName := ""
		// method
		switch v := fnDecl.Recv.List[0].Type.(type) {
		case *ast.Ident:
			typeName = v.Name
		case *ast.StarExpr:
			ident, ok := v.X.(*ast.Ident)
			if ok {
				typeName = ident.Name
			} else {
				log.Fatalf("unexpected ast type %T", v.X)
			}

		default:
			log.Fatalf("[WARN] unhandled method reciver case %T\n", v)

		}
		if strings.HasSuffix(typeName, MacroSymbol) {
			methods[fmt.Sprintf("%s.%s", typeName, fnDecl.Name.Name)] = fnDecl
		}
	}
	return methods
}

func AllMacroDecl(f *ast.File, allMacroDecl map[string]*ast.FuncDecl) {
	for _, decl := range f.Decls {
		fnDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fnDecl.Recv == nil {
			if strings.HasSuffix(fnDecl.Name.Name, MacroSymbol) {
				allMacroDecl[fmt.Sprintf("%s.%s", f.Name, fnDecl.Name.Name)] = fnDecl
				// load without lib prefix
				allMacroDecl[fnDecl.Name.Name] = fnDecl
			}
			continue
		}
		typeName := ""
		// method
		switch v := fnDecl.Recv.List[0].Type.(type) {
		case *ast.Ident:
			typeName = v.Name
		case *ast.StarExpr:
			ident, ok := v.X.(*ast.Ident)
			if ok {
				typeName = ident.Name
			} else {
				log.Fatalf("unexpected ast type %T", v.X)
			}

		default:
			log.Fatalf("[WARN] unhandled method reciver case %T\n", v)

		}
		if strings.HasSuffix(typeName, MacroSymbol) {
			allMacroDecl[fmt.Sprintf("%s.%s.%s", f.Name, typeName, fnDecl.Name.Name)] = fnDecl
			allMacroDecl[fmt.Sprintf("%s.%s", typeName, fnDecl.Name.Name)] = fnDecl
		}
	}
}

func createCallExpr(fun ast.Expr, args []ast.Expr) *ast.CallExpr {
	expr := &ast.CallExpr{
		Fun:  fun,
		Args: args,
	}
	return expr
}

func createIfErrRetStmt(err ast.Expr, ret ast.Expr) *ast.IfStmt {
	stmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: err, Op: token.NEQ, Y: &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{ret},
				},
			},
		},
	}
	return stmt
}

func createAssignStmt(lhs, rhs []ast.Expr, tok token.Token) *ast.AssignStmt {
	stmt := &ast.AssignStmt{
		Lhs: lhs, Tok: tok, Rhs: rhs,
	}
	return stmt
}

// fnNameFromCallExpr returns name of func/method call
// from ast.CallExpr
// TODO test with closure()().Method and arr[i].Param.Method calls
func FnNameFromCallExpr(fn *ast.CallExpr) (string, error) {
	var err error
	var fname string
	var combineName func(*ast.SelectorExpr) (string, error)

	combineName = func(expr *ast.SelectorExpr) (string, error) {
		switch v := expr.X.(type) {
		case *ast.Ident:
			// base case
			return v.Name + "." + expr.Sel.Name, nil
		case *ast.SelectorExpr:
			fname, err := combineName(v)
			return fname + "." + expr.Sel.Name, err
		case *ast.CallExpr:
			fname, err := FnNameFromCallExpr(v)
			return fname + "." + expr.Sel.Name, err
		default:
			fmt.Printf("combine: unexpected AST %# v\n", pretty.Formatter(v)) // output for debug
			out, err := FormatNode(v)
			fmt.Printf("Err node print err %v out %+v\n", err, out) // output for debug

			return "", fmt.Errorf("unexpected value %T", v)
		}
	}

	switch v := fn.Fun.(type) {
	case *ast.Ident:
		// base case
		fname = v.Name
	case *ast.SelectorExpr:
		fname, err = combineName(v)
		if err != nil {
			return "", err
		}
	default:
		fmt.Printf("unexpected AST %# v\n", pretty.Formatter(v)) // output for debug
		return "", fmt.Errorf("unexpected value %T", v)
	}

	return fname, nil
}

// TODO define better name, creates only assignment statements
// shallow copy with new assignable statements
func copyBodyStmt(argNum int, body *ast.BlockStmt, noreturns bool) *ast.BlockStmt {
	block := new(ast.BlockStmt)
	block.Lbrace = body.Lbrace
	block.Rbrace = body.Rbrace
	block.List = make([]ast.Stmt, 0, len(body.List))
	for _, st := range body.List {
		if _, ok := st.(*ast.ReturnStmt); ok && noreturns {
			// skip returns
			continue
		}
		// copy
		block.List = append(block.List, st)
	}
	// TODO handle return on macros returning func
	if len(body.List) == 0 || !noreturns {
		return block
	}

	for i := 0; i < argNum; i++ {
		st := block.List[i]
		// it should be first elements in the list
		if assignStmt, ok := st.(*ast.AssignStmt); ok {
			cloneStmt := &ast.AssignStmt{
				Lhs:    assignStmt.Lhs,
				TokPos: assignStmt.TokPos,
				Tok:    assignStmt.Tok,
				Rhs:    make([]ast.Expr, len(assignStmt.Rhs)),
			}
			block.List[i] = cloneStmt
		}
	}
	return block
}

// creates var {name} {typ};
// returns identifier created
func newDeclStmt(decTyp token.Token, name string, typ ast.Expr) (*ast.DeclStmt, *ast.Ident) {
	stmt := new(ast.DeclStmt)
	genDecl := new(ast.GenDecl)
	genDecl.Tok = decTyp
	valSpec := new(ast.ValueSpec)
	ident := &ast.Ident{
		Name: name,
		Obj: &ast.Object{
			Kind: objKindToTokenType(decTyp),
			Name: name,
			Decl: valSpec,
		},
	}
	valSpec.Names = append(valSpec.Names, ident)
	valSpec.Type = typ
	genDecl.Specs = []ast.Spec{valSpec}
	stmt.Decl = genDecl
	return stmt, ident
}

// TODO check with packages
func IdentsFromCallExpr(idents *[]*ast.Ident, callArgs *[][]ast.Expr, expr *ast.CallExpr) {
	switch v := expr.Fun.(type) {
	case *ast.Ident:
		*idents = append(*idents, v)
	case *ast.SelectorExpr:
		switch X := v.X.(type) {
		case *ast.Ident:
			*idents = append(*idents, X)
		case *ast.CallExpr:
			IdentsFromCallExpr(idents, callArgs, X)
		default:
			log.Fatalf("selector unsupported type %T %# v\n", v, pretty.Formatter(v))
		}
		*idents = append(*idents, v.Sel)
	default:
		log.Fatalf("default unsupported type %T\n", v)
	}
	*callArgs = append(*callArgs, expr.Args)
}

// TODO rename
// returns copy of func literal
func getFuncLit(decl *ast.FuncDecl) (*ast.FuncLit, bool) {
	for _, st := range decl.Body.List {
		if ret, ok := st.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
			// only one function to return
			if fn, ok := ret.Results[0].(*ast.FuncLit); ok {
				return copyFuncLit(fn), true
			}
		}
	}
	return nil, false
}

func copyFuncLit(fn *ast.FuncLit) *ast.FuncLit {
	cfn := new(ast.FuncLit)
	cfn.Type = fn.Type // function type
	// handle only one return statement
	cfn.Body = copyBodyStmt(0, fn.Body, false)
	return cfn
}

func objKindToTokenType(typ token.Token) ast.ObjKind {
	switch typ {
	case token.VAR:
		return ast.Var
	default:
		log.Fatalf("unexpected type %+v", typ)
		return ast.Bad
	}
}

func IsMacroDecl(decl *ast.FuncDecl) bool {
	if decl == nil {
		return false
	}
	if decl.Recv == nil {
		return strings.HasSuffix(decl.Name.Name, MacroSymbol)
	}
	// method
	fld := decl.Recv.List[0]
	typeName := ""
	switch v := fld.Type.(type) {
	case *ast.Ident:
		typeName = v.Name
	case *ast.StarExpr:
		ident, ok := v.X.(*ast.Ident)
		if ok {
			typeName = ident.Name
		} else {
			log.Fatalf("unexpected ast type %T", v.X)
		}
	default:
		fmt.Printf("[WARN] unhandled method reciver case %T\n", v)
	}

	return strings.HasSuffix(typeName, MacroSymbol)
}

func checkIsMacroIdent(name string, idents []*ast.Ident) bool {
	if len(idents) == 0 {
		return false
	}
	// check if it's log macro
	if (idents[0].Name == "macro" && idents[1].Name == name) ||
		idents[0].Name == name {
		return true

	}
	return false
}

func FormatNode(node ast.Node) (string, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), node)
	if err != nil {
		fmt.Printf("AST on error %+v\n", pretty.Formatter(node)) // output for debug
	}
	return buf.String(), err
}

// general rules
// TODO describe rules
func MacroGeneralExpand(
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	var newSeqBlocks []ast.Stmt
	var blocks []ast.Stmt

	for i := 0; i < len(idents); i++ {
		ident := idents[i]
		if !strings.HasSuffix(ident.Name, MacroSymbol) {
			continue
		}
		// check if ident has return type
		// TODO what to do if obj literal used? Prohibit from constructing
		// by unexported field?
		var funDecl *ast.FuncDecl
		var ok bool
		if funDecl, ok = ident.Obj.Decl.(*ast.FuncDecl); !ok {
			log.Fatalf("funDecl expected but got %+v", ident.Obj.Decl)
		}
		// TODO refactor what checks needed
		body := copyBodyStmt(len(callArgs[i]),
			funDecl.Body, true)
		// find all body args defined as assignments
		var bodyArgs []*ast.AssignStmt
		for _, ln := range body.List {
			if st, ok := ln.(*ast.AssignStmt); ok {
				bodyArgs = append(bodyArgs, st)
			}
		}

		// TODO check that number of args is correct
		// switch Rhs with call args
		// TODO support multiple declaration in one line
		for i, carg := range callArgs[i] {
			bodyArgs[i].Rhs = []ast.Expr{carg}
		}
		// expand body macros
		astutil.Apply(body, pre, post)
		blocks = append(blocks, body)

	}
	if len(blocks) > 0 {
		blockStmt := new(ast.BlockStmt)
		blockStmt.Lbrace = cur.Node().End()
		if newSeqBlocks != nil {
			blockStmt.List = append(blockStmt.List, newSeqBlocks...)
			newSeqBlocks = nil
		}
		blockStmt.List = append(blockStmt.List, blocks...)
		// insert as one block
		cur.InsertAfter(blockStmt)
		cur.Delete()
	}

	return true
}
