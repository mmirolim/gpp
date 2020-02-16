package macro

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"regexp"
	"strings"

	"github.com/kr/pretty"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

const (
	MacroSymbol     = "_μ"
	Seq_μTypeSymbol = "seq_μ"
	NewSeq_μSymbol  = "NewSeq_μ"
	Try_μSymbol     = "Try_μ"
	Log_μSymbol     = "Log_μ"
	MacroPkgPath    = "github.com/mmirolim/gpp/macro"
	MacroPkgName    = "macro"
)

// TODO move to context?
var ApplyState = struct {
	MacroLibName string
	RemoveLib    bool
	File         *ast.File
	Fset         *token.FileSet
	Pkg          *packages.Package
	SrcDir       string
	LogRe        *regexp.Regexp
	IsOuterMacro bool
}{}

// define custom macro expand functions
// TODO make settable, prefixed by modulename?
var MacroExpanders = map[string]MacroExpander{
	Seq_μTypeSymbol: MacroNewSeq,
	Try_μSymbol:     MacroTryExpand,
	Log_μSymbol:     MacroLogExpand,
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
	parentStmt, callExpr := getCallExprAndParent(n)
	if callExpr == nil {
		return true
	}
	var callArgs [][]ast.Expr
	var idents []*ast.Ident
	IdentsFromCallExpr(callExpr, &idents, &callArgs)
	if len(idents) == 0 {
		// skip unhandled cases
		return true
	}

	// skip lib prefix
	if idents[0].Name == ApplyState.MacroLibName {
		idents = idents[1:]
	}

	if !strings.HasSuffix(idents[0].Name, MacroSymbol) && idents[0].Obj != nil {
		// resolve if var is in local scope
		if stmt, ok := idents[0].Obj.Decl.(*ast.AssignStmt); ok {
			newIdent, newCallArgs := resolveVarInLocalScope(idents[0].Name, stmt)
			if newIdent != nil {
				// use var pos for new ident
				newIdent.NamePos = idents[0].Pos()
				idents[0] = newIdent
				if strings.HasSuffix(newIdent.Name, MacroSymbol) {
					ApplyState.RemoveLib = false
				}
			}
			if newCallArgs != nil {
				callArgs = append([][]ast.Expr{newCallArgs}, callArgs...)
			}
		}
	}

	decl := getMacroDeclByName(idents[0].Name)
	if decl == nil {
		return true
	}
	macroTypeName := getFirstTypeInReturn(decl)
	ident := idents[0]
	ident.Obj = &ast.Object{Name: ident.Name, Decl: decl}
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

func resolveVarInLocalScope(identName string, stmt *ast.AssignStmt) (ident *ast.Ident, args []ast.Expr) {
	id := 0
	numberOfMuted := 0
	for i, expr := range stmt.Lhs {
		if lhsIdent, ok := expr.(*ast.Ident); ok {
			// original ident name
			if lhsIdent.Obj.Name == identName {
				id = i
			}
			if lhsIdent.Name == "_" {
				numberOfMuted++
			}
		}
	}
	switch expr := stmt.Rhs[id].(type) {
	case *ast.SelectorExpr:
		ident = expr.Sel
	case *ast.Ident:
		ident = expr
	default:
		// unhandled
	}

	if ident != nil {
		if muteIdent, ok := stmt.Lhs[id].(*ast.Ident); ok {
			muteIdent.Name = "_"
		}
		// change assign symbol if all muted
		if numberOfMuted+1 == len(stmt.Lhs) {
			stmt.Tok = token.ASSIGN
		}
	}
	return
}

func getFirstTypeInReturn(decl ast.Decl) string {
	if decl == nil {
		return ""
	}
	if fnDecl, ok := decl.(*ast.FuncDecl); ok {
		// TODO construct for not only star expressions or any selector?
		if fnDecl.Type.Results != nil && fnDecl.Type.Results.List[0] != nil {
			expr, ok := fnDecl.Type.Results.List[0].Type.(*ast.StarExpr)
			if ok {
				//  TODO use recursive solution
				id := expr.X.(*ast.Ident)
				return id.Name
			}
		}
	}
	return ""
}

func getCallExprAndParent(n ast.Node) (parentStmt ast.Stmt, callExpr *ast.CallExpr) {
	switch stmt := n.(type) {
	case *ast.ExprStmt:
		// as standalone expr
		if cexp, ok := stmt.X.(*ast.CallExpr); ok {
			parentStmt = stmt
			callExpr = cexp
		}
	case *ast.AssignStmt:
		// in assignment
		for i := range stmt.Rhs {
			if cexp, ok := stmt.Rhs[i].(*ast.CallExpr); ok {
				parentStmt = stmt
				callExpr = cexp
			}
		}
	}
	return
}

func getMacroDeclByName(name string) *ast.FuncDecl {
	if fdecl, ok := MacroDecl[name]; ok {
		return fdecl
	}
	return nil
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

// copyBodyStmt only creates assignment statements
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
func IdentsFromCallExpr(expr *ast.CallExpr, idents *[]*ast.Ident, callArgs *[][]ast.Expr) {
	switch v := expr.Fun.(type) {
	case *ast.Ident:
		*idents = append(*idents, v)
	case *ast.SelectorExpr:
		switch X := v.X.(type) {
		case *ast.Ident:
			*idents = append(*idents, X)
		case *ast.CallExpr:
			IdentsFromCallExpr(X, idents, callArgs)
		default:
			log.Fatalf("selector unsupported type %T %# v\n", v, pretty.Formatter(v))
		}
		*idents = append(*idents, v.Sel)
	case *ast.IndexExpr:
		idents = nil
		return // does not support macro from index expr
	case *ast.FuncLit:
		// skip
	default:
		// TODO indirections
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
