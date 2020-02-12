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
	dst      = flag.String("C", ".", "working directory")
	runFlag  = flag.Bool("run", false, "run run binary")
	testFlag = flag.Bool("test", false, "test binary")
	goArgs   = flag.String("args", "", "args to go")

	// temp directory to use
	src = filepath.Join(os.TempDir(), "gpp_temp_build_dir")
)

func main() {
	flag.Parse()
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd error %+v", err)
	}
	base := filepath.Base(curDir)
	src := filepath.Join(src, base)

	// clean temp directory with source code
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
	args := strings.Split(*goArgs, " ")
	if *testFlag {
		cmd = exec.Command("go", "test", "-v", "./...")
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("%+v\n", cmd.Args) // output for debug
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatalf("binary exec error %+v", err)
		}
	} else {
		// go build
		cmd = exec.Command("go", "build")
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("%+v\n", cmd.Args) // output for debug
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatalf("go build error %+v", err)
		}
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
		cmd = exec.Command("./" + base)
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("%+v\n", cmd.Args) // output for debug
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatalf("binary exec error %+v", err)
		}
	}
}

func parseDir(dir string) error {
	ctx := context.Background()
	cfg := &packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
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
			if ctx.Err() != nil {
				fmt.Fprintln(os.Stderr, "task canceled")
				fmt.Fprintln(os.Stderr, "\n============================")
				err = errors.New("task canceled")
				return err
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
			modifiedAST := astutil.Apply(file, macro.Pre, macro.Post)
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
