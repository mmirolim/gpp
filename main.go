package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mmirolim/gpp/macro"
	"golang.org/x/tools/go/ast/astutil"
)

var (
	dst = flag.String("C", ".", "working directory")
	src = "/dev/shm/gm"
)

// Test macros as library
// Test parsing whole application
// go run/build
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
	macroMethods = macro.AllMacroMethods(file)
	state := &macro.ApplyState
	state.IsOuterMacro = false
	state.File = file
	state.Fset = fset
	out := astutil.Apply(file, pre, post)
	astStr, err := macro.FormatNode(out)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, []byte(astStr), 0700)
	if err != nil {
		return err
	}
	return err
}

func pre(cur *astutil.Cursor) bool {
	n := cur.Node()
	// on macro define do not expand it
	if funDecl, ok := n.(*ast.FuncDecl); ok {
		macro.ApplyState.IsOuterMacro = macro.IsMacroDecl(funDecl)
	}
	// process AssignStmt

	if macro.ApplyState.IsOuterMacro {
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
	// apply macro expand rules
	var callArgs [][]ast.Expr
	var idents []*ast.Ident

	macro.IdentsFromCallExpr(&idents, &callArgs, callExpr)
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
				if strings.HasSuffix(id.Name, macro.MacroSymbol) {
					macroTypeName = id.Name
				}
			}
		}
	}

	// get expand func
	if expand, ok := macro.MacroExpanders[macroTypeName]; ok {
		expand(cur, parentStmt, idents, callArgs, pre, post)
	} else if expand, ok := macro.MacroExpanders[ident.Name]; ok {
		expand(cur, parentStmt, idents, callArgs, pre, post)
	} else if strings.HasSuffix(ident.Name, macro.MacroSymbol) {
		macro.MacroGeneralExpand(cur, parentStmt, idents, callArgs, pre, post)
	}

	return true
}

func post(cur *astutil.Cursor) bool {
	return true
}
