package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"io/fs"
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
	runFlag  = flag.Bool("run", false, "run the built binary")
	testFlag = flag.Bool("test", false, "test binary")
	goArgs   = flag.String("args", "", "args to go")
	logFlag  = flag.String("log", "", "regex matching filename:line")
	diffFlag = flag.Bool("diff", false, "show macro expansion diff without building")
	logRe    *regexp.Regexp
)

func main() {
	flag.Parse()
	if *logFlag != "" {
		logRe = regexp.MustCompile(*logFlag)
	}

	workDir, err := filepath.Abs(*dst)
	if err != nil {
		log.Fatalf("resolving work directory: %+v", err)
	}

	curDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd error: %+v", err)
	}

	moduleName, err := getModuleName(workDir)
	if err != nil {
		log.Fatalf("getModuleName: %+v", err)
	}

	// Create staging directory with only the files needed for building.
	// This avoids copying the entire project (no .git, binaries, data files, etc.)
	// and leaves the original source untouched.
	stagingDir, err := os.MkdirTemp("", "gpp-build-*")
	if err != nil {
		log.Fatalf("creating staging directory: %+v", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(stagingDir)
		}
	}()

	// Copy Go module files and source files to staging
	if err := copyModuleToStaging(workDir, stagingDir); err != nil {
		log.Fatalf("copying to staging: %+v", err)
	}

	// --diff mode: show original vs expanded code without building
	if *diffFlag {
		if err := showDiff(stagingDir, workDir, moduleName); err != nil {
			log.Fatalf("diff error: %+v", err)
		}
		cleanup = false
		os.RemoveAll(stagingDir)
		cleanup = false
		return
	}

	// Parse and expand macros in the staging directory
	if err := parseDir(stagingDir, moduleName, logRe); err != nil {
		log.Fatalf("parse dir error: %+v", err)
	}

	// Build or test from the staging directory using proper Go module support
	args := splitArgs(*goArgs)
	if *testFlag {
		cmd := exec.Command("go", "test", "-v", "./...")
		cmd.Args = append(cmd.Args, args...)
		cmd.Dir = stagingDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("go test error: %+v", err)
		}
	} else {
		binaryName := filepath.Base(moduleName)
		outputPath := filepath.Join(curDir, binaryName)
		cmd := exec.Command("go", "build", "-o", outputPath)
		cmd.Args = append(cmd.Args, args...)
		cmd.Dir = stagingDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("go build error: %+v", err)
		}
	}

	cleanup = false // keep staging on failure path is handled by defer

	if *runFlag && !*testFlag {
		binaryName := filepath.Base(moduleName)
		cmd := exec.Command("./" + binaryName)
		cmd.Args = append(cmd.Args, args...)
		cmd.Dir = curDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("binary exec error: %+v", err)
		}
	}

	// Everything succeeded, clean up staging
	os.RemoveAll(stagingDir)
	cleanup = false
}

// showDiff reads original files, expands macros, and prints a unified-style diff.
func showDiff(stagingDir, origDir, moduleName string) error {
	// Parse and expand macros in the staging directory
	if err := parseDir(stagingDir, moduleName, logRe); err != nil {
		return err
	}

	// Walk the staging directory and compare with originals
	foundDiffs := false
	err := filepath.WalkDir(stagingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		rel, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}

		// Skip files in the macro library itself
		if strings.Contains(rel, "macro/") {
			return nil
		}

		expanded, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		original, err := os.ReadFile(filepath.Join(origDir, rel))
		if err != nil {
			// New file (e.g., nooplog stub injection), show it
			fmt.Fprintf(os.Stderr, "\n=== NEW: %s ===\n%s\n", rel, string(expanded))
			foundDiffs = true
			return nil
		}

		if string(original) != string(expanded) {
			if !foundDiffs {
				fmt.Fprintln(os.Stderr, "")
				foundDiffs = true
			}
			fmt.Fprintf(os.Stderr, "=== DIFF: %s ===\n", rel)
			printDiff(string(original), string(expanded), rel)
			fmt.Fprintln(os.Stderr, "")
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !foundDiffs {
		fmt.Fprintln(os.Stderr, "No macro expansions found. Code is unchanged after preprocessing.")
	}
	return nil
}

// printDiff prints a simple line-by-line diff between original and expanded.
func printDiff(original, expanded, filename string) {
	origLines := strings.Split(original, "\n")
	expLines := strings.Split(expanded, "\n")

	maxLen := len(origLines)
	if len(expLines) > maxLen {
		maxLen = len(expLines)
	}

	for i := 0; i < maxLen; i++ {
		var origLine, expLine string
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(expLines) {
			expLine = expLines[i]
		}

		if origLine != expLine {
			if origLine != "" && i < len(origLines) {
				fmt.Fprintf(os.Stderr, "\033[31m- %s:%d: %s\033[0m\n", filename, i+1, origLine)
			}
			if expLine != "" && i < len(expLines) {
				fmt.Fprintf(os.Stderr, "\033[32m+ %s:%d: %s\033[0m\n", filename, i+1, expLine)
			}
		}
	}
}

// copyModuleToStaging copies only the files needed for building:
// go.mod, go.sum, *.go files, and symlinks vendor/ if present.
// This is much faster and cleaner than copying the entire project.
func copyModuleToStaging(srcDir, stagingDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip hidden directories, test output, and vendor (handled separately)
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", ".hg", ".svn", ".gpp-cache":
				return filepath.SkipDir
			case "vendor":
				// Symlink vendor instead of copying
				return os.Symlink(path, filepath.Join(stagingDir, rel))
			}
			return nil
		}

		// Only copy files needed for Go module building
		base := filepath.Base(path)
		switch {
		case base == "go.mod":
			// Copy go.mod with resolved absolute replace directives
			return copyGoModWithAbsReplaces(path, filepath.Join(stagingDir, rel))
		case base == "go.sum":
			return copyFile(path, filepath.Join(stagingDir, rel))
		case strings.HasSuffix(base, ".go"):
			return copyFile(path, filepath.Join(stagingDir, rel))
		case strings.HasSuffix(base, ".s"), strings.HasSuffix(base, ".c"), strings.HasSuffix(base, ".h"):
			// CGO support files
			return copyFile(path, filepath.Join(stagingDir, rel))
		}
		return nil
	})
}

// copyGoModWithAbsReplaces copies go.mod and resolves any relative replace
// directives to absolute paths so they work from the staging directory.
func copyGoModWithAbsReplaces(srcMod, dstMod string) error {
	if err := os.MkdirAll(filepath.Dir(dstMod), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(srcMod)
	if err != nil {
		return err
	}
	// Resolve relative replace paths from the go.mod file's original directory
	goModDir := filepath.Dir(srcMod)
	var out strings.Builder
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for replace directive with relative path
		if strings.HasPrefix(trimmed, "replace ") && strings.Contains(trimmed, "=>") {
			parts := strings.SplitN(trimmed, "=>", 2)
			if len(parts) == 2 {
				replacePath := strings.TrimSpace(parts[1])
				// Check if it's a relative path (not absolute, not a version like @v1.0)
				if !filepath.IsAbs(replacePath) && !strings.HasPrefix(replacePath, "@") {
					absPath, err := filepath.Abs(filepath.Join(goModDir, replacePath))
					if err == nil {
						line = parts[0] + "=> " + absPath
					}
				}
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return os.WriteFile(dstMod, []byte(out.String()), 0o644)
}

// copyFile copies a single file, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func parseDir(dir, moduleName string, logRe *regexp.Regexp) error {
	bgCtx := context.Background()
	cfg := &packages.Config{
		Context: bgCtx,
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
			if bgCtx.Err() != nil {
				fmt.Fprintln(os.Stderr, "task canceled")
				fmt.Fprintln(os.Stderr, "\n============================")
				return errors.New("task canceled")
			}
			packages.PrintErrors(pkgs)
			fmt.Fprintln(os.Stderr, "\n============================")
			return errors.New("packages.Load error")
		}
	}
	var visitFailed bool
	var loadMacroLibOnce sync.Once
	macroDecls := make(map[string]*ast.FuncDecl)
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

			// Create per-file context instead of using global state
			macroCtx := &macro.Context{
				File:         file,
				Fset:         pkg.Fset,
				Pkg:          pkg,
				SrcDir:       dir,
				LogRe:        logRe,
				RemoveLib:    true,
				MacroLibName: getMacroLibName(file),
				MacroDecls:   macroDecls,
			}

			if macroPkg, ok := pkg.Imports[macro.MacroPkgPath]; ok {
				loadMacroLibOnce.Do(func() {
					for _, file := range macroPkg.Syntax {
						macro.AllMacroDecl(file, macroDecls)
					}
				})
			} else {
				return true // no macro in package
			}

			// remove comments
			file.Comments = nil
			modifiedAST := astutil.Apply(file, macro.NewPre(macroCtx), macro.NewPost(macroCtx))
			updatedFile := modifiedAST.(*ast.File)
			if macroCtx.RemoveLib {
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
			err = os.WriteFile(pkg.GoFiles[i], []byte(astStr), 0o700)
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

// getModuleName returns module name from go.mod in workDir.
func getModuleName(workDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("go.mod not found in %s: %w", workDir, err)
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

// splitArgs splits a space-separated argument string, ignoring empty parts.
func splitArgs(s string) []string {
	var args []string
	for _, a := range strings.Split(s, " ") {
		if a != "" {
			args = append(args, a)
		}
	}
	return args
}
