package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func Log_μ(args ...interface{}) {
}

// TODO add fmt package if missing
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
		Kind: token.STRING,
		Value: fmt.Sprintf("%s:%d ",
			strings.TrimPrefix(fileInfo.Name(), ApplyState.SrcDir),
			fileInfo.Line(idents[0].Pos())),
	}
	var args []ast.Expr
	args = append(args, fmtCfg)
	for _, carg := range callArgs[0] {
		switch v := carg.(type) {
		case *ast.BasicLit:
			fmtCfg.Value += "%v "
		case *ast.Ident:
			fmtCfg.Value += fmt.Sprintf("%s=%%#v ", v.Name)
		case *ast.CallExpr:
			callName, err := FnNameFromCallExpr(v)
			if err == nil {
				fmtCfg.Value += fmt.Sprintf("%v=%%#v ", callName)
			} else {
				fmtCfg.Value += fmt.Sprintf("%T=%%#v ", v)
			}
		default:
			// define type of unknown
			fmtCfg.Value += fmt.Sprintf("%T=%%#v ", v)
			continue
		}
		args = append(args, carg)
	}
	if len(fmtCfg.Value) > 0 {
		fmtCfg.Value = fmtCfg.Value[0 : len(fmtCfg.Value)-1]
	}
	fmtCfg.Value = fmt.Sprintf("\"%s\\n\"", fmtCfg.Value)
	callExpr := createCallExpr(fmtExpr, args)
	cur.InsertAfter(&ast.ExprStmt{X: callExpr})
	// expand body macros
	astutil.Apply(callExpr, pre, post)
	cur.Delete()
	return true
}
