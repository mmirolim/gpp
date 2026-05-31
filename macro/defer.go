package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Defer_μ wraps a cleanup call in a defer with error handling.
// Normally `defer f.Close()` silently ignores the error.
// Defer_μ expands to a defer that logs the error from the cleanup call.
//
//	f, _ := os.Open("file.txt")
//	macro.Defer_μ(f.Close)
//
// Expands to:
//
//	defer func() {
//	    if err := f.Close(); err != nil {
//	        log.Printf("gpp defer f.Close: %v", err)
//	    }
//	}()
func Defer_μ(fn any) {}

const Defer_μSymbol = "Defer_μ"

// MacroDeferExpand is the expander for Defer_μ.
func MacroDeferExpand(
	ctx *Context,
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if !checkIsMacroIdent(Defer_μSymbol, idents) {
		return false
	}
	if len(callArgs) == 0 || len(callArgs[0]) == 0 {
		return false
	}

	// The argument should be a function call like f.Close
	cleanupCall := callArgs[0][0]

	// Get the call name for the error message
	callName := "cleanup"
	if formatted, err := FormatNode(cleanupCall); err == nil {
		callName = formatted
		// Trim the () from the name
		if idx := strings.LastIndexByte(callName, '('); idx > 0 {
			callName = callName[:idx]
		}
	}

	errIdent := ast.NewIdent("err")

	// If the argument is a call expression (f.Close()), use it directly.
	// If it's a selector expression (f.Close), wrap it in a call.
	var cleanupCallExpr ast.Expr
	if _, ok := cleanupCall.(*ast.CallExpr); ok {
		cleanupCallExpr = cleanupCall
	} else {
		// Wrap f.Close → f.Close()
		cleanupCallExpr = createCallExpr(cleanupCall, nil)
	}

	// err := f.Close()
	checkAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{errIdent},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{cleanupCallExpr},
	}

	// log.Printf("gpp defer f.Close: %v", err)
	logExpr := &ast.SelectorExpr{
		X:   ast.NewIdent("log"),
		Sel: ast.NewIdent("Printf"),
	}
	logCall := createCallExpr(logExpr, []ast.Expr{
		&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("\"gpp defer %s: %%v\"", callName)},
		errIdent,
	})

	// if err != nil { log.Printf(...) }
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  ast.NewIdent("err"),
			Op: token.NEQ,
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{&ast.ExprStmt{X: logCall}},
		},
	}

	// defer func() { ... }()
	deferLit := &ast.FuncLit{
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{checkAssign, ifStmt},
		},
	}
	deferCall := createCallExpr(deferLit, nil)
	deferStmt := &ast.DeferStmt{
		Call: deferCall,
	}

	// Replace the macro.Defer_μ(f.Close) call with the expanded defer
	cur.InsertAfter(deferStmt)
	cur.Delete()

	// Add "log" import if not already present
	astutil.AddImport(ctx.Fset, ctx.File, "log")

	return true
}

func init() {
	MacroExpanders[Defer_μSymbol] = MacroDeferExpand
}
