package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mmirolim/gpp/macro"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

var (
	dst     = flag.String("C", ".", "working directory")
	src     = filepath.Join(os.TempDir(), "gpp_temp_build_dir")
	runFlag = flag.Bool("run", false, "build run binary")
)

// Test macros as library
// Test parsing whole application
// go run/build
func main() {
	flag.Parse()
	// parse file
	// generate correct AST to insert
	// pass to compiler
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd error %+v", err)
	}
	base := filepath.Base(curDir)
	src := filepath.Join(src, base)
	// clean prev directory
	err = os.RemoveAll(src)
	if err != nil {
		log.Fatalf("remove all error %+v", err)
	}
	err = os.MkdirAll(src, 0700)
	if err != nil {
		log.Fatalf("mkdir all error %+v", err)
	}
	// copy whole directory to tmp dir
	cmd := exec.Command("cp", "-r", *dst, src)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("cp -r error %+v", err)
	}
	// change dir
	err = os.Chdir(src)
	if err != nil {
		log.Fatalf("chdir %+v", err)
	}
	err = parseDir(src)
	if err != nil {
		log.Fatalf("parse dir error %+v", err)
	}
	// go build
	cmd = exec.Command("go", "build")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatalf("go build error %+v", err)
	}

	err = os.Chdir(curDir)
	if err != nil {
		log.Fatalf("chdir %+v", err)
	}
	// copy binary back
	cmd = exec.Command("cp", filepath.Join(src, base), base)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("cp error %+v", err)
	}
	if *runFlag {
		// TODO pass flags
		cmd = exec.Command("./" + base)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatalf("binary exec error %+v", err)
		}
	}
}

// packages should be vendored otherwise original lib/deps files will be
// overwritten
func parseDir(dir string) error {
	ctx := context.Background()
	cfg := &packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedImports |
			packages.NeedDeps,
		Tests: true,
	}
	var err error
	var pkgs []*packages.Package
	// find all packages
	pkgs, err = packages.Load(cfg, "./...")
	if err != nil {
		return err
	}

	for i := range pkgs {
		if len(pkgs[i].Errors) > 0 {
			fmt.Fprintln(os.Stderr, "\n=======\033[31m Build Failed \033[39m=======")
			select {
			case <-ctx.Done():
				fmt.Fprintln(os.Stderr, "task canceled")
				err = errors.New("task canceled")
				return err
			default:
			}
			packages.PrintErrors(pkgs)
			fmt.Fprintln(os.Stderr, "\n============================")
			err = errors.New("packages.Load error")
			return err
		}
	}

	var visitFailed bool
	var loadMacroLibOnce sync.Once
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if visitFailed {
			// skip imported packages on pkg fail
			return true
		}
		for i, file := range pkg.Syntax {
			if macroPkg, ok := pkg.Imports[macro.MacroPkgPath]; ok {
				loadMacroLibOnce.Do(func() {
					for _, file := range macroPkg.Syntax {
						macro.AllMacroDecl(file, macro.MacroDecl)
					}
				})
			} else {
				return true // no macro in package
			}

			removeMacroLibImport(file)
			// remove comments
			file.Comments = nil
			macro.ApplyState.IsOuterMacro = false
			macro.ApplyState.File = file
			macro.ApplyState.Fset = pkg.Fset
			macro.ApplyState.Pkg = pkg
			macro.ApplyState.SrcDir = src
			modifiedAST := astutil.Apply(file, pre, post)
			updatedFile := modifiedAST.(*ast.File)
			astStr, err := macro.FormatNode(updatedFile)
			if err != nil {
				fmt.Printf("format node err %+v\n", err) // output for debug
				visitFailed = true
				break
			}
			// packages should be vendored otherwise original lib/deps files will be
			// overwritten
			err = ioutil.WriteFile(pkg.GoFiles[i], []byte(astStr), 0700)
			if err != nil {
				fmt.Printf("write error %+v\n", err) // output for debug
				visitFailed = true
				break
			}
		}
		return true
	}, nil)

	return nil
}

func removeMacroLibImport(file *ast.File) {
	for di, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for i := range genDecl.Specs {
			spec, ok := genDecl.Specs[i].(*ast.ImportSpec)
			if !ok {
				continue
			}
			if !strings.Contains(spec.Path.Value, macro.MacroPkgPath) {
				continue
			}
			if len(genDecl.Specs) == 1 {
				// remove import decl
				file.Decls = append(file.Decls[:di], file.Decls[di+1:]...)
			} else {
				genDecl.Specs = append(genDecl.Specs[:i], genDecl.Specs[i+1:]...)
			}
			return
		}
	}
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
		if strings.HasSuffix(ident.Name, macro.MacroSymbol) {
			ident.Obj = &ast.Object{
				Name: ident.Name,
				Decl: macro.MacroDecl[ident.Name],
			}
		} else if ident.Name == "macro" {
			name := fmt.Sprintf("%s.%s", ident.Name, idents[1].Name)
			ident.Obj = &ast.Object{
				Name: name,
				Decl: macro.MacroDecl[name],
			}
		} else {
			return true
		}
	}

	macroTypeName := ""
	if decl, ok := ident.Obj.Decl.(*ast.FuncDecl); ok {
		// TODO construct for not only star expressions
		// can be selector?
		if decl != nil && decl.Type.Results != nil && decl.Type.Results.List[0] != nil {
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
