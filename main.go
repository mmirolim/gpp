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

var (
	dst = flag.String("C", ".", "working directory")
	src = "/dev/shm/gm"
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

	//fmt.Printf("AST %# v\n", pretty.Formatter(pkgs)) // output for debug
	var file *ast.File
	var fileName string
	for fname := range pkgs["main"].Files {
		fileName = fname
		file = pkgs["main"].Files[fname]
		break
	}
	macroMethods = allMacroMethods(file)
	for k := range macroMethods {
		fmt.Printf("macro method %+v\n", k) // output for debug

	}
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

const macrosymbol = "_μ"

// TODO should resolve across packages
func allMacroMethods(f *ast.File) map[string]*ast.FuncDecl {
	methods := make(map[string]*ast.FuncDecl)
	for _, decl := range f.Decls {
		if fnDecl, ok := decl.(*ast.FuncDecl); ok {
			if fnDecl.Recv != nil {
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
		}
	}
	return methods
}

var applyState = struct {
	isOuterMacro bool
}{}

func pre(cur *astutil.Cursor) bool {
	n := cur.Node()
	// on macro define do not expand it
	if funDecl, ok := n.(*ast.FuncDecl); ok {
		isMacro := false
		if funDecl.Recv != nil {
			// method
			fld := funDecl.Recv.List[0]
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
				fmt.Printf("[WARN] unhandled method reciver case %T\n", v) // output for debug

			}
			fmt.Printf("TypeName found %+v\n", typeName) // output for debug

			isMacro = strings.HasSuffix(typeName, "_μ")
		} else {
			fmt.Printf("FunDecl found %+v\n", funDecl.Name.Name) // output for debug
			isMacro = strings.HasSuffix(funDecl.Name.Name, "_μ")
		}
		applyState.isOuterMacro = isMacro
	}
	if estmt, ok := n.(*ast.ExprStmt); ok && !applyState.isOuterMacro {
		if cexp, ok := estmt.X.(*ast.CallExpr); ok {
			var callArgs [][]ast.Expr
			var idents []*ast.Ident
			identsFromCallExpr(&idents, &callArgs, cexp)
			fmt.Printf("Args %d Idents %d\n", len(callArgs), len(idents)) // output for debug
			//fmt.Printf("Call Args %# v\n", pretty.Formatter(callArgs))    // output for debug

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
					} else {
						log.Fatalf("wrong type for results %T", expr)
					}
				}
			}
			var blocks []ast.Stmt
			for i := len(idents) - 1; i > -1; i-- {
				//fmt.Printf("Ident %# v\n", pretty.Formatter(ident)) // output for debug
				ident := idents[i]
				if !strings.HasSuffix(ident.Name, "_μ") {
					continue
				}
				fmt.Printf("Macro found  %+v\n", ident.Name) // output for debug
				// check if ident has return type
				// TODO what to do if obj literal used? Prohibit from constructing
				// by unexported field?

				//fmt.Printf("Ident %# v\n", pretty.Formatter(ident)) // output for debug

				var funDecl *ast.FuncDecl
				if ident.Obj == nil && macroTypeName != "" {
					funDecl = macroMethods[fmt.Sprintf("%s.%s", macroTypeName, ident.Name)]
				} else {
					var ok bool
					funDecl, ok = ident.Obj.Decl.(*ast.FuncDecl)
					if !ok {
						log.Fatalf("funDecl expected but got %+v", ident.Obj.Decl)
					}
				}

				if funDecl != nil { //
					fmt.Printf("Macro name expand %+v\n", ident.Name) // output for debug
					body := copyBodyStmt(len(callArgs[i]), funDecl.Body, true)
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
					if strings.HasPrefix(funDecl.Name.Name, "New") {
						blocks = append(blocks, body.List...)
					} else {
						blocks = append(blocks, body)
					}

				}

			}
			if len(blocks) > 0 {
				for i := range blocks {
					cur.InsertAfter(blocks[i])
				}
				cur.Delete()
			}

		}

	}
	return true
}

func post(cur *astutil.Cursor) bool {
	return true
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
	if len(body.List) == 0 {
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
func fnNameFromCallExpr(fn *ast.CallExpr) string {
	var fname string
	var combineName func(*ast.SelectorExpr) string

	combineName = func(expr *ast.SelectorExpr) string {
		switch v := expr.X.(type) {
		case *ast.Ident:
			// base case
			return v.Name + "." + expr.Sel.Name
		case *ast.SelectorExpr:
			return combineName(v) + "." + expr.Sel.Name
		case *ast.CallExpr:
			return fnNameFromCallExpr(v) + "." + expr.Sel.Name
		default:
			fmt.Printf("combine: unexpected AST %# v\n", pretty.Formatter(v)) // output for debug
			out, err := FormatNode(v)
			fmt.Printf("Err node print err %v out %+v\n", err, out) // output for debug

			log.Fatalf("unexpected value %T", v)
			return ""
		}
	}

	switch v := fn.Fun.(type) {
	case *ast.Ident:
		// base case
		fname = v.Name
	case *ast.SelectorExpr:
		fname = combineName(v)
	default:
		fmt.Printf("unexpected AST %# v\n", pretty.Formatter(v)) // output for debug
		log.Fatalf("unexpected value %T", v)
	}

	return fname
}

func FormatNode(node ast.Node) (string, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), node)
	if err != nil {
		fmt.Printf("AST on error %+v\n", pretty.Formatter(node)) // output for debug
	}
	return buf.String(), err
}
