package macro

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

func Log_μ(args ...interface{}) {
}

func MacroLogExpand(
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if !checkIsMacroIdent(Log_μSymbol, idents) {
		return false
	}
	if len(callArgs[0]) == 0 {
		return false
	}
	// TODO expected that it important
	// construct fmt.Printf()
	fmtExpr := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "fmt"},
		Sel: &ast.Ident{Name: "Printf"},
	}
	pos := idents[0].Pos()
	fileInfo := ApplyState.Fset.File(pos)
	fmtCfg := &ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("%s:%d\\n", fileInfo.Name(), fileInfo.Line(idents[0].Pos())),
	}
	var args []ast.Expr
	args = append(args, fmtCfg)
	for _, carg := range callArgs[0] {
		switch v := carg.(type) {
		case *ast.BasicLit:
			fmtCfg.Value += "%v\\n"
		case *ast.Ident:
			fmtCfg.Value += fmt.Sprintf("%s=%%#v\\n", v.Name)
		case *ast.CallExpr:
			callName, err := FnNameFromCallExpr(v)
			if err == nil {
				fmtCfg.Value += fmt.Sprintf("%v=%%#v\\n", callName)
			} else {
				fmtCfg.Value += fmt.Sprintf("%T=%%#v\\n", v)
			}
		default:
			// define type of unknown
			fmtCfg.Value += fmt.Sprintf("%T=%%#v\\n", v)
			continue
		}
		args = append(args, carg)
	}
	fmtCfg.Value = fmt.Sprintf("\"%s\"", fmtCfg.Value)
	callExpr := createCallExpr(fmtExpr, args)
	cur.InsertAfter(&ast.ExprStmt{X: callExpr})
	// expand body macros
	astutil.Apply(callExpr, pre, post)
	cur.Delete()
	return true
}
