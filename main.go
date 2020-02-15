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
	"regexp"
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
	logFlag  = flag.String("log", "", "regex matching filename:line")
	// temp directory to use
	gopath = filepath.Join(os.TempDir(), "gpp_temp_build_dir", "go")
	logRe  *regexp.Regexp
)

func main() {
	flag.Parse()
	if *logFlag != "" {
		logRe = regexp.MustCompile(*logFlag)
	}
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd error %+v", err)
	}
	moduleName, err := getModuleName(curDir)
	if err != nil {
		log.Fatalf("getModuleName %+v", err)
	}
	// source code path according to modulename
	src := filepath.Join(gopath, "src", moduleName)
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
	err = parseDir(src, moduleName, logRe)
	if err != nil {
		log.Fatalf("parse dir error %+v", err)
	}
	// set gopath for cmd
	envs := append(os.Environ(), "GOPATH="+gopath)
	args := strings.Split(*goArgs, " ")
	if *testFlag {
		cmd = exec.Command("go", "test", "-v", "./...")
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = envs
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
		cmd.Env = envs
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
	base := filepath.Base(src)
	cmd = exec.Command("cp", filepath.Join(src, base), base)
	err = cmd.Run()
	if err != nil {
		log.Fatalf("cp error %+v", err)
	}
	if *runFlag {
		cmd = exec.Command("./" + base)
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = envs
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatalf("binary exec error %+v", err)
		}
	}
}

func parseDir(dir, moduleName string, logRe *regexp.Regexp) error {
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
	if logRe != nil {
		// insert nooplog stub
		insertNoOpLogStub(pkgs)
	}
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		// TODO do it in parallel
		if visitFailed {
			// skip imported packages on pkg fail
			return true
		}

		for i, file := range pkg.Syntax {
			// skip non local files
			// TODO check net package have more pkg.Syntax than pkg.GoFiles
			if i < len(pkg.GoFiles) && !strings.HasPrefix(pkg.GoFiles[i], dir) {
				continue
			}

			macro.ApplyState.IsOuterMacro = false
			macro.ApplyState.File = file
			macro.ApplyState.Fset = pkg.Fset
			macro.ApplyState.Pkg = pkg
			macro.ApplyState.SrcDir = dir
			macro.ApplyState.LogRe = logRe
			macro.ApplyState.RemoveLib = true
			macro.ApplyState.MacroLibName = getMacroLibName(file)

			if macroPkg, ok := pkg.Imports[macro.MacroPkgPath]; ok {
				loadMacroLibOnce.Do(func() {
					for _, file := range macroPkg.Syntax {
						macro.AllMacroDecl(file, macro.MacroDecl)
					}
				})
			} else {
				return true // no macro in package
			}

			// remove comments
			file.Comments = nil
			modifiedAST := astutil.Apply(file, macro.Pre, macro.Post)
			updatedFile := modifiedAST.(*ast.File)
			if macro.ApplyState.RemoveLib {
				removeMacroLibImport(updatedFile)
			}
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

func insertNoOpLogStub(pkgs []*packages.Package) {
	for _, pkg := range pkgs {
		file := pkg.Syntax[0]
		// and inject to decl
		decl := macro.CreateNoOpFuncDecl(macro.LogFuncStubName)
		file.Decls = append(file.Decls, decl)
	}
}

// getModuleName returns module name
// in gived workDir
func getModuleName(workDir string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		// get from GOPATH
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			return "", errors.New("GOPATH and go.mod not found")
		}
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Rel(filepath.Join(gopath, "src"), dir)
	}

	var line []byte
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			break
		}
		line = data[0 : i+1]
	}

	return strings.Split(string(line), " ")[1], nil
}

// getMacroLibName returns name of macro library in import
func getMacroLibName(file *ast.File) string {
	macroLibPath := fmt.Sprintf("\"%s\"", macro.MacroPkgPath)
	for _, imprt := range file.Imports {
		if imprt.Path.Value == macroLibPath {
			if imprt.Name != nil {
				return imprt.Name.Name
			}
			return macro.MacroPkgName
		}
	}
	return ""
}
