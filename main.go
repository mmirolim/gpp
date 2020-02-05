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

func parseDir(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return err
	}

	fmt.Printf("AST %# v\n", pretty.Formatter(pkgs)) // output for debug
	var file *ast.File
	var fileName string
	for fname := range pkgs["main"].Files {
		fileName = fname
		file = pkgs["main"].Files[fname]
		break
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

func pre(cur *astutil.Cursor) bool {
	n := cur.Node()
	if estmt, ok := n.(*ast.ExprStmt); ok {
		if cexp, ok := estmt.X.(*ast.CallExpr); ok {
			fnName, err := fnNameFromCallExpr(cexp)
			if err != nil {
				log.Fatal(err)
			}
			if fnName != "printMap_μ" {
				return true
			}
			fmt.Printf("Found macro >>%+v<<\n", "printMap_μ") // output for debug
			if funIdent, ok := cexp.Fun.(*ast.Ident); ok {
				if funDecl, ok := funIdent.Obj.Decl.(*ast.FuncDecl); ok {
					fmt.Printf("%+v\n", "insert body of printmap") // output for debug
					body := copyBodyStmt(funDecl.Body)
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
					for i, carg := range cexp.Args {
						bodyArgs[i].Rhs = []ast.Expr{carg}
					}
					cur.InsertAfter(body)
					cur.Delete()
				}
			}
		}
	}
	return true
}

func post(cur *astutil.Cursor) bool {
	return true
}

// TODO define better name, creates only assignment statements
// shallow copy with new assignable statements
func copyBodyStmt(body *ast.BlockStmt) *ast.BlockStmt {
	block := new(ast.BlockStmt)
	block.Lbrace = body.Lbrace
	block.Rbrace = body.Rbrace
	block.List = make([]ast.Stmt, len(body.List))
	for i, st := range body.List {
		// copy
		block.List[i] = st
	}
	for i, st := range body.List {
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
func fnNameFromCallExpr(fn *ast.CallExpr) (string, error) {
	var fname string
	var err error
	var combineName func(*ast.SelectorExpr) string

	combineName = func(expr *ast.SelectorExpr) string {
		switch v := expr.X.(type) {
		case *ast.Ident:
			// base case
			return v.Name + "." + expr.Sel.Name
		case *ast.SelectorExpr:
			return combineName(v) + "." + expr.Sel.Name
		default:
			err = fmt.Errorf("unexpected value %T", v)
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
		err = fmt.Errorf("unexpected value %T", v)
	}

	return fname, err
}

func FormatNode(node ast.Node) (string, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), node)
	if err != nil {
		fmt.Printf("AST on error %+v\n", pretty.Formatter(node)) // output for debug
	}
	return buf.String(), err
}

var a = func() int {
	return 10
}()

var slice = []int{1, 2, 3, 4, 5, 6, 7}

var out = seq(slice).
	Map(func(v int) int { return v * 2 }).
	Filter(func(v int) bool { return v%2 == 0 }).
	Map(func(v int) int {
		fmt.Println(v)
		return v
	}).Get()

type _T int
type _Seq struct{ seq []_T }

func seq(sl interface{}) *_Seq {
	var arg1 []_T
	return &_Seq{seq: arg1}
}

func (sq *_Seq) Filter(fn interface{}) *_Seq {
	var arg1 func(_T) bool
	var out1 []_T
	for i := range sq.seq {
		if arg1(sq.seq[i]) {
			out1 = append(out1, sq.seq[i])
		}
	}
	sq.seq = out1
	return sq
}

func (sq *_Seq) Map(fn interface{}) *_Seq {
	var arg1 func(_T) _T
	for i := range sq.seq {
		sq.seq[i] = arg1(sq.seq[i])
	}
	return sq
}

func (sq *_Seq) Get() interface{} {
	return sq.seq
}
