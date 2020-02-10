package macro

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

func Try_μ(fn interface{}) error {
	return nil
}

// TODO gtr didn't find test to run for this function
// Define rules to expand, like func signature, last lhs is err
// which is checked and so on
// TODO Wrap errors with callexpr names to be able to identify it
func MacroTryExpand(
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if len(idents) > 0 && idents[0].Name != Try_μSymbol {
		return false
	}

	if len(callArgs[0]) == 0 {
		return false
	}
	// func lit is arg of try
	funcLit, ok := callArgs[0][0].(*ast.FuncLit)
	if !ok {
		fmt.Printf("WARN expected Try macro, got %+v\n", callArgs[0]) // output for debug
		return false
	}
	// expected assignstmt
	pstmt := parentStmt.(*ast.AssignStmt)
	// check all errors
	var bodyList []ast.Stmt
	// create new err variable
	errDecl, errIdent := newDeclStmt(token.VAR, "err", &ast.Ident{Name: "error"})
	bodyList = append(bodyList, errDecl)
	for _, stmt := range funcLit.Body.List {
		bodyList = append(bodyList, stmt)
		// only assignment handled
		// expr stmt problem to resolve during parsing what it returns
		if assignStmt, ok := stmt.(*ast.AssignStmt); ok {
			// expect last unused variable to be an error
			lastVar := assignStmt.Lhs[len(assignStmt.Lhs)-1].(*ast.Ident)
			if lastVar.Name != "_" {
				continue
			}
			// check is callExpr
			var cexp *ast.CallExpr
			if cexp, ok = assignStmt.Rhs[0].(*ast.CallExpr); !ok {
				continue
			}

			// replace with err
			assignStmt.Lhs[len(assignStmt.Lhs)-1] = errIdent
			callName, _ := fnNameFromCallExpr(cexp)
			fmtCfg := &ast.BasicLit{
				Kind:  token.STRING,
				Value: fmt.Sprintf("\"%s: %%w\"", callName),
			}
			fmtExpr := &ast.SelectorExpr{
				X:   &ast.Ident{Name: "fmt"},
				Sel: &ast.Ident{Name: "Errorf"},
			}
			callExpr := createCallExpr(fmtExpr, []ast.Expr{fmtCfg, errIdent})
			bodyList = append(bodyList, createIfErrRetStmt(errIdent, callExpr))
		}
	}
	funcLit.Body.List = bodyList
	// last element should be return
	ret, ok := bodyList[len(bodyList)-1].(*ast.ReturnStmt)
	if ok {
		ret.Results[0] = errIdent
	}
	callExpr := createCallExpr(funcLit, nil)
	pstmt.Rhs = []ast.Expr{callExpr}
	// expand body macros
	astutil.Apply(callExpr, pre, post)

	return true
}
