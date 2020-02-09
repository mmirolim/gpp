package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/kr/pretty"
	"golang.org/x/tools/go/ast/astutil"
)

const (
	macrosymbol = "_μ"
	seq_μType   = "seq_μ"
)

var (
	dst = flag.String("C", ".", "working directory")
	src = "/dev/shm/gm"
	// define custom macro expand functions
	// TODO make settable
	macroExpanders = map[string]MacroExpander{
		seq_μType: MacroNewSeq,
	}
)

func main() {
	flag.Parse()
	fmt.Printf("%+v\n", "go macro experiment") // output for debug
	// parse file
	// generate correct AST to insert
	// pass to compiler

	// clean prev directory
	err := os.RemoveAll(src)
	if err != nil {
		log.Fatal(err)
	}
	// copy whole directory to /dev/shm
	fmt.Printf("%+v\n", "copy to tmp dir") // output for debug
	cmd := exec.Command("cp", "-r", *dst, src)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	// change dir
	err = os.Chdir(src)
	if err != nil {
		log.Fatal(err)
	}

	err = parseDir(src)
	if err != nil {
		log.Fatal(err)
	}

	// go build
	cmd = exec.Command("go", "build")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

var macroMethods map[string]*ast.FuncDecl

func parseDir(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return err
	}

	var file *ast.File
	var fileName string
	// TODO process all files
	for fname := range pkgs["main"].Files {
		fileName = fname
		file = pkgs["main"].Files[fname]
		break
	}
	macroMethods = allMacroMethods(file)

	out := astutil.Apply(file, pre, post)
	astStr, err := FormatNode(out)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, []byte(astStr), 0700)
	if err != nil {
		return err
	}
	return err
}

// TODO should resolve across packages
func allMacroMethods(f *ast.File) map[string]*ast.FuncDecl {
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
		if strings.HasSuffix(typeName, macrosymbol) {
			methods[fmt.Sprintf("%s.%s", typeName, fnDecl.Name.Name)] = fnDecl
		}
	}
	return methods
}

var applyState = struct {
	isOuterMacro bool
}{}

func isMacroDecl(decl *ast.FuncDecl) bool {
	if decl == nil {
		return false
	}
	if decl.Recv == nil {
		return strings.HasSuffix(decl.Name.Name, "_μ")
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

	return strings.HasSuffix(typeName, "_μ")
}

func pre(cur *astutil.Cursor) bool {
	n := cur.Node()
	// on macro define do not expand it
	if funDecl, ok := n.(*ast.FuncDecl); ok {
		applyState.isOuterMacro = isMacroDecl(funDecl)
	}
	estmt, ok := n.(*ast.ExprStmt)
	if ok && !applyState.isOuterMacro {
		// apply macro expand rules
		if cexp, ok := estmt.X.(*ast.CallExpr); ok {
			var callArgs [][]ast.Expr
			var idents []*ast.Ident
			identsFromCallExpr(&idents, &callArgs, cexp)
			ident := idents[0]
			if ident.Obj == nil || ident.Obj.Decl == nil {
				return true
			}
			macroTypeName := ""
			if decl, ok := ident.Obj.Decl.(*ast.FuncDecl); ok {
				// TODO construct for not only star expressions
				// can be selector?
				if decl.Type.Results != nil && decl.Type.Results.List[0] != nil {
					expr, ok := decl.Type.Results.List[0].Type.(*ast.StarExpr)
					if ok {
						//  TODO use recursive solution
						id := expr.X.(*ast.Ident)
						if strings.HasSuffix(id.Name, macrosymbol) {
							macroTypeName = id.Name
						}
					}
				}
			}

			// get expand func
			if expand, ok := macroExpanders[macroTypeName]; ok {
				expand(cur, idents, callArgs, pre)
			} else if expand, ok := macroExpanders[ident.Name]; ok {
				expand(cur, idents, callArgs, pre)
			} else {
				macroGeneralExpand(cur, idents, callArgs, pre)
			}
		}
	}

	return true
}

func post(cur *astutil.Cursor) bool {
	return true
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
func identsFromCallExpr(idents *[]*ast.Ident, callArgs *[][]ast.Expr, expr *ast.CallExpr) {
	switch v := expr.Fun.(type) {
	case *ast.Ident:
		*idents = append(*idents, v)
	case *ast.SelectorExpr:
		switch X := v.X.(type) {
		case *ast.Ident:
			*idents = append(*idents, X)
		case *ast.CallExpr:
			identsFromCallExpr(idents, callArgs, X)
		default:
			log.Fatalf("selector unsupported type %T %# v\n", v, pretty.Formatter(v))
		}
		*idents = append(*idents, v.Sel)
	default:
		log.Fatalf("default unsupported type %T\n", v)
	}
	*callArgs = append(*callArgs, expr.Args)
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

// fnNameFromCallExpr returns name of func/method call
// from ast.CallExpr
// TODO test with closure()().Method and arr[i].Param.Method calls
func fnNameFromCallExpr(fn *ast.CallExpr) (string, error) {
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
			fname, err := fnNameFromCallExpr(v)
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

func FormatNode(node ast.Node) (string, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), node)
	if err != nil {
		fmt.Printf("AST on error %+v\n", pretty.Formatter(node)) // output for debug
	}
	return buf.String(), err
}

type MacroExpander func(*astutil.Cursor, []*ast.Ident, [][]ast.Expr, astutil.ApplyFunc) bool

type MacroCtxKey string

const (
	NewSeqKey MacroCtxKey = "NewSeqKey"
)

// special rules
func MacroNewSeq(
	cur *astutil.Cursor,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre astutil.ApplyFunc) bool {

	var newSeqBlocks []ast.Stmt
	var lastNewSeqStmt ast.Stmt
	var lastNewSeqSeq *ast.Ident
	var blocks []ast.Stmt

	for i := 0; i < len(idents); i++ {
		ident := idents[i]
		// check if ident has return type
		// TODO what to do if obj literal used? Prohibit from constructing
		// by unexported field?
		var funDecl *ast.FuncDecl
		if ident.Obj == nil {
			funDecl = macroMethods[fmt.Sprintf("%s.%s", seq_μType, ident.Name)]
			if funDecl == nil {
				fmt.Printf("WARN Method Decl not found %+v\n", ident.Name) // output for debug

				continue
			}

		} else {
			var ok bool
			funDecl, ok = ident.Obj.Decl.(*ast.FuncDecl)
			if !ok {
				log.Fatalf("funDecl expected but got %+v", ident.Obj.Decl)
			}
		}
		if ident.Name != "NewSeq_μ" && newSeqBlocks != nil {
			// TODO refactor what checks needed
			// create decl state for storing sequence
			funcLit, _ := callArgs[i][0].(*ast.FuncLit)
			// add to new block
			prevNewSeqBlock := newSeqBlocks[len(newSeqBlocks)-1]
			var prevObj *ast.Object
			switch val := prevNewSeqBlock.(type) {
			case *ast.AssignStmt:
				prevObj = val.Lhs[0].(*ast.Ident).Obj
			case *ast.DeclStmt:
				prevObj = lastNewSeqSeq.Obj
			}

			// assign ident to input
			callArgs[i] = append(callArgs[i], &ast.Ident{
				Name: fmt.Sprintf("%s%d", "seq", i-1),
				Obj:  prevObj,
			})
			if ident.Name != "Get" && ident.Name != "Reduce" {
				var resultTyp ast.Expr
				if ident.Name == "Map" {
					resultTyp = funcLit.Type.Results.List[0].Type
				} else if ident.Name == "Filter" {
					resultTyp = funcLit.Type.Params.List[0].Type
				}
				arrType := &ast.ArrayType{
					Elt: resultTyp,
				}
				lastNewSeqStmt, lastNewSeqSeq = newDeclStmt(
					token.VAR, fmt.Sprintf("%s%d", "seq", i),
					arrType)
				newSeqBlocks = append(newSeqBlocks, lastNewSeqStmt)

				// assing unary op to output
				callArgs[i] = append(callArgs[i], &ast.UnaryExpr{
					Op: token.AND,
					X: &ast.Ident{
						Name: fmt.Sprintf("%s%d", "seq", i),
						Obj:  lastNewSeqSeq.Obj,
					},
				})
			}
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
		// TODO support multiple declaration in one line
		for i, carg := range callArgs[i] {
			bodyArgs[i].Rhs = []ast.Expr{carg}
		}
		// expand body macros
		astutil.Apply(body, pre, post)
		// New funcs which returns macro type should have parent scope
		if strings.HasPrefix(funDecl.Name.Name, "New") {
			newSeqBlocks = body.List
		} else {
			blocks = append(blocks, body)
		}

	}

	if len(blocks) > 0 {
		blockStmt := new(ast.BlockStmt)
		blockStmt.Lbrace = cur.Node().End()
		if newSeqBlocks != nil {
			blockStmt.List = append(blockStmt.List, newSeqBlocks...)
			newSeqBlocks = nil
			lastNewSeqStmt = nil
			lastNewSeqSeq = nil
		}
		blockStmt.List = append(blockStmt.List, blocks...)
		// insert as one block
		cur.InsertAfter(blockStmt)
		cur.Delete()
	}

	return true
}

// general rules
// TODO describe rules
func macroGeneralExpand(
	cur *astutil.Cursor,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre astutil.ApplyFunc) bool {
	var newSeqBlocks []ast.Stmt
	var blocks []ast.Stmt

	for i := 0; i < len(idents); i++ {
		ident := idents[i]
		if !strings.HasSuffix(ident.Name, "_μ") {
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
