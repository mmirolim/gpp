package macro

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"strings"

	"github.com/kr/pretty"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
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
	Pkg          *packages.Package
	SrcDir       string
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

// MacroExpander expander function type
type MacroExpander func(cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc,
) bool

// PrintSlice_μ --
func PrintSlice_μ(sl interface{}) {
	arg1 := []_T{}
	for i := range arg1 {
		fmt.Printf("%v\n", arg1[i])
	}
}

// AllMacroDecl collects func decl of macros
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

// Pre ApplyFunc for ast processing
func Pre(cur *astutil.Cursor) bool {
	n := cur.Node()
	if funDecl, ok := n.(*ast.FuncDecl); ok {
		ApplyState.IsOuterMacro = IsMacroDecl(funDecl)
	}
	// do not expand in macro func declarations
	if ApplyState.IsOuterMacro {
		return false
	}
	var parentStmt ast.Stmt
	var callExpr *ast.CallExpr
	// as standalone expr
	if estmt, ok := n.(*ast.ExprStmt); ok {
		if cexp, ok := estmt.X.(*ast.CallExpr); ok {
			parentStmt = estmt
			callExpr = cexp
		}
	}
	// in assignment
	if assign, ok := n.(*ast.AssignStmt); ok {
		for i := range assign.Rhs {
			if cexp, ok := assign.Rhs[i].(*ast.CallExpr); ok {
				parentStmt = assign
				callExpr = cexp
			}
		}

	}

	if callExpr == nil {
		return true
	}
	var callArgs [][]ast.Expr
	var idents []*ast.Ident
	IdentsFromCallExpr(&idents, &callArgs, callExpr)
	if len(idents) == 0 {
		// skip unhandled cases
		return true
	}
	// first ident
	ident := idents[0]
	// skip lib prefix
	if ident.Name == "macro" {
		idents = idents[1:]
		ident = idents[0]
	}

	if ident.Obj == nil || ident.Obj.Decl == nil {
		// TODO define func
		// check in macro decls
		if strings.HasSuffix(ident.Name, MacroSymbol) {
			ident.Obj = &ast.Object{
				Name: ident.Name,
				Decl: MacroDecl[ident.Name],
			}
		} else if ident.Name == "macro" {
			name := fmt.Sprintf("%s.%s", ident.Name, idents[1].Name)
			ident.Obj = &ast.Object{
				Name: name,
				Decl: MacroDecl[name],
			}
		} else {
			return true
		}
	}

	macroTypeName := ""
	if decl, ok := ident.Obj.Decl.(*ast.FuncDecl); ok {
		// TODO construct for not only star expressions or any selector?
		if decl != nil && decl.Type.Results != nil && decl.Type.Results.List[0] != nil {
			expr, ok := decl.Type.Results.List[0].Type.(*ast.StarExpr)
			if ok {
				//  TODO use recursive solution
				id := expr.X.(*ast.Ident)
				if strings.HasSuffix(id.Name, MacroSymbol) {
					macroTypeName = id.Name
				}
			}
		}
	}
	// get expand func
	if expand, ok := MacroExpanders[macroTypeName]; ok {
		expand(cur, parentStmt, idents, callArgs, Pre, Post)
	} else if expand, ok := MacroExpanders[ident.Name]; ok {
		expand(cur, parentStmt, idents, callArgs, Pre, Post)
	} else if strings.HasSuffix(ident.Name, MacroSymbol) {
		MacroGeneralExpand(cur, parentStmt, idents, callArgs, Pre, Post)
	}

	return true
}

// Post ApplyFunc for ast processing
func Post(cur *astutil.Cursor) bool {
	return true
}

// MacroGeneralExpand default expander
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
		var funDecl *ast.FuncDecl
		var ok bool
		// TODO handle other types, use Seq resolver
		if funDecl, ok = ident.Obj.Decl.(*ast.FuncDecl); !ok {
			fmt.Printf("WARN MacroGeneralExpand funDecl expected but got %+v\n", ident.Obj.Decl)
			continue
		}
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

// FnNameFromCallExpr returns name of func/method call
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
			out, err := FormatNode(v)
			fmt.Printf("Err node print err %v out %+v\n", err, out)
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

// copyBOdyStmt only creates assignment statements
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
func createDeclStmt(decTyp token.Token, name string, typ ast.Expr) (*ast.DeclStmt, *ast.Ident) {
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

// IdentsFromCallExpr
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
		// TODO indexes, indirections
		fmt.Printf("WARN IdentsFromCallExpr unsuported type %T\n", v) // output for debug
		return
	}
	*callArgs = append(*callArgs, expr.Args)
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

// FormatNode format to text
func FormatNode(node ast.Node) (string, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), node)
	if err != nil {
		fmt.Printf("AST on error %+v\n", pretty.Formatter(node)) // output for debug
	}
	return buf.String(), err
}

// resolveExpr create obj with func declaration from expr signature
// TODO rename
func resolveExpr(expr ast.Expr, curPkg *packages.Package) *ast.Object {
	if sig, ok := curPkg.TypesInfo.TypeOf(expr).(*types.Signature); ok {
		return &ast.Object{
			Name: "resolved_func_template",
			Decl: &ast.FuncDecl{
				Type: createFuncTypeFromSignature(sig, curPkg),
			},
		}
	}
	fmt.Printf("WARN resolveExpr unhandled expr to resolve %# v\n", pretty.Formatter(expr))
	fmt.Printf("WARN TypeOf(expr) %#v\n", pretty.Formatter(curPkg.TypesInfo.TypeOf(expr)))
	return nil
}

func createFuncTypeFromSignature(sig *types.Signature, curPkg *packages.Package) *ast.FuncType {
	ft := &ast.FuncType{}
	params := sig.Params()
	paramList := make([]*ast.Field, 0, params.Len())
	// TODO check is there other way around
	getVarTyp := func(v *types.Var) string {
		typ := v.Type().String()
		idx := strings.LastIndexByte(typ, '/')
		if idx > -1 {
			ch := typ[0]
			typ = typ[idx+1:]
			if curPkg.Name == v.Pkg().Name() {
				idx = strings.LastIndexByte(typ, '.')
				if idx > -1 {
					typ = typ[idx+1:]
				}
			}
			if ch == '*' {
				typ = "*" + typ
			}
		}
		return typ
	}
	for i := 0; i < params.Len(); i++ {
		paramList = append(paramList, &ast.Field{
			Names: []*ast.Ident{
				{Name: fmt.Sprintf("a%d", i)}, // ignored
			},
			Type: &ast.Ident{
				Name: getVarTyp(params.At(i)),
			},
		})
	}
	results := sig.Results()
	resultList := make([]*ast.Field, 0, results.Len())
	for i := 0; i < results.Len(); i++ {
		resultList = append(resultList, &ast.Field{
			Names: []*ast.Ident{
				{Name: fmt.Sprintf("r%d", i)}, // ignored
			},
			Type: &ast.Ident{
				Name: getVarTyp(results.At(i)),
			},
		})
	}
	ft.Params = &ast.FieldList{List: paramList}
	ft.Results = &ast.FieldList{List: resultList}
	return ft
}

func resolveIdentInPkg(ident *ast.Ident, pkg *packages.Package) *ast.Object {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			funDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if funDecl.Name.Name == ident.Name {
				return funDecl.Name.Obj
			}
		}
	}
	return nil
}
