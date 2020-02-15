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

// LogFuncStubName used as stub to mute unmatched log lines
const LogFuncStubName = "__nooplog_"

// MacroLogExpand transformer for Log_μ
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
	pos := idents[0].Pos()
	fileInfo := ApplyState.Fset.File(pos)
	fileAndPos := fmt.Sprintf("%s:%d ",
		strings.TrimPrefix(fileInfo.Name(), ApplyState.SrcDir),
		fileInfo.Line(idents[0].Pos()))

	// if enabled check match
	if ApplyState.LogRe != nil && !ApplyState.LogRe.MatchString(fileAndPos) {
		// remove
		if stmt, ok := cur.Node().(*ast.ExprStmt); ok {
			if callExpr, ok := stmt.X.(*ast.CallExpr); ok {
				if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
					selExpr.Sel.Name = LogFuncStubName
					callExpr.Fun = selExpr.Sel
				}
			}
		}
		return false
	}
	// construct fmt.Printf()
	fmtExpr := &ast.SelectorExpr{
		// TODO handle when import renamed
		X:   &ast.Ident{Name: "fmt"},
		Sel: &ast.Ident{Name: "Printf"},
	}
	fmtCfg := &ast.BasicLit{
		Kind:  token.STRING,
		Value: fileAndPos,
	}
	var args []ast.Expr
	args = append(args, fmtCfg)
	for _, carg := range callArgs[0] {
		switch v := carg.(type) {
		case *ast.BasicLit:
			fmtCfg.Value += "%v "
		default:
			callName, err := FormatNode(v)
			if err != nil {
				fmt.Printf("WARN FormatNode error on type %T\n", v)
			}
			callName = strings.ReplaceAll(callName, "\"", "'")
			fmtCfg.Value += fmt.Sprintf("%s=%%#v ", callName)
		}
		args = append(args, carg)
	}
	if len(fmtCfg.Value) > 0 {
		fmtCfg.Value = fmtCfg.Value[0 : len(fmtCfg.Value)-1]
	}
	fmtCfg.Value = fmt.Sprintf("\"%s\\n\"", fmtCfg.Value)
	callExpr := createCallExpr(fmtExpr, args)
	cur.InsertAfter(&ast.ExprStmt{X: callExpr})
	astutil.AddImport(ApplyState.Fset, ApplyState.File, "fmt")

	// expand body macros
	astutil.Apply(callExpr, pre, post)
	cur.Delete()
	return true
}

func CreateNoOpFuncDecl(name string) *ast.FuncDecl {
	decl := &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("args")},
						Type: &ast.Ellipsis{
							Elt: &ast.InterfaceType{
								Methods: &ast.FieldList{},
							},
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{},
	}
	return decl
}
