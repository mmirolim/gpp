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

const tryErrName = "_tryerr_"

// MacroTryExpand try macro expander
func MacroTryExpand(
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if !checkIsMacroIdent(Try_μSymbol, idents) {
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

	// create new err variable
	errDecl, errIdent := createDeclStmt(token.VAR, tryErrName, &ast.Ident{Name: "error"})
	var procRecur func([]ast.Stmt) []ast.Stmt
	depth := 0
	// check all errors in all statements recursively
	procRecur = func(stmts []ast.Stmt) []ast.Stmt {
		depth++
		var bodyList []ast.Stmt
		var cexp *ast.CallExpr
		var assignStmt *ast.AssignStmt
	OUTER:
		for _, stmt := range stmts {
			bodyList = append(bodyList, stmt)
			switch rstmt := stmt.(type) {
			case *ast.AssignStmt:
				// error expected to be last ignored return value
				lastVar, ok := rstmt.Lhs[len(rstmt.Lhs)-1].(*ast.Ident)
				if !ok {
					continue OUTER
				}
				if lastVar.Name != "_" {
					continue OUTER
				}
				// check is callExpr
				if cexp, ok = rstmt.Rhs[0].(*ast.CallExpr); !ok {
					continue OUTER
				}

				obj := resolveExpr(cexp.Fun, ApplyState.Pkg)
				funcDecl := obj.Decl.(*ast.FuncDecl)
				// check if it is error
				lastReturnType := funcDecl.Type.Results.
					List[len(funcDecl.Type.Results.List)-1].Type
				if typIdent, ok := lastReturnType.(*ast.Ident); ok {
					if typIdent.Name != "error" {
						continue OUTER
					}
				} else {
					fmt.Printf("WARN Try macro unexpected type %T\n", lastReturnType)
					continue OUTER

				}
				assignStmt = rstmt
			case *ast.ExprStmt:
				if cexp, ok = rstmt.X.(*ast.CallExpr); !ok {
					continue OUTER
				}
				obj := resolveExpr(cexp.Fun, ApplyState.Pkg)
				funcDecl := obj.Decl.(*ast.FuncDecl)
				// check if it is error
				if len(funcDecl.Type.Results.List) == 0 {
					continue OUTER // does not return anything
				}
				lastReturnType := funcDecl.Type.Results.
					List[len(funcDecl.Type.Results.List)-1].Type
				if typIdent, ok := lastReturnType.(*ast.Ident); ok {
					if typIdent.Name != "error" {
						continue OUTER
					}
				} else {
					fmt.Printf("WARN Try macro unexpected type %T\n", lastReturnType)
					continue OUTER

				}
				// balance assignment
				var lhs []ast.Expr
				for i := 0; i < len(funcDecl.Type.Results.List); i++ {
					lhs = append(lhs, &ast.Ident{Name: "_"})
				}
				rhs := []ast.Expr{cexp}
				assignStmt = createAssignStmt(lhs, rhs, token.ASSIGN)
				// replace current statement
				bodyList[len(bodyList)-1] = assignStmt
			default:
				// TODO handle other block statements if/else/for etc
				switch instmt := stmt.(type) {
				case *ast.CaseClause:
					instmt.Body = procRecur(instmt.Body)
				case *ast.CommClause:
					instmt.Body = procRecur(instmt.Body)
				case *ast.ForStmt:
					instmt.Body.List = procRecur(instmt.Body.List)
				case *ast.IfStmt:
					instmt.Body.List = procRecur(instmt.Body.List)
				case *ast.RangeStmt:
					instmt.Body.List = procRecur(instmt.Body.List)
				case *ast.SelectStmt:
					instmt.Body.List = procRecur(instmt.Body.List)
				case *ast.SwitchStmt:
					instmt.Body.List = procRecur(instmt.Body.List)
				case *ast.TypeSwitchStmt:
					instmt.Body.List = procRecur(instmt.Body.List)
				default:
					// skip
				}
				// unhandled statements
				continue OUTER

			}
			// replace with err
			if len(assignStmt.Lhs) > 0 {
				assignStmt.Lhs[len(assignStmt.Lhs)-1] = errIdent
			} else {
				assignStmt.Lhs = []ast.Expr{errIdent}
			}

			callName, _ := FnNameFromCallExpr(cexp)
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
		return bodyList
	}
	// add top level var err decl
	stmts := []ast.Stmt{errDecl}
	stmts = append(stmts, procRecur(funcLit.Body.List)...)

	// last element should be return
	ret, ok := stmts[len(stmts)-1].(*ast.ReturnStmt)
	if ok {
		ret.Results[0] = errIdent
	}
	funcLit.Body.List = stmts
	callExpr := createCallExpr(funcLit, nil)
	pstmt.Rhs = []ast.Expr{callExpr}
	// expand body macros
	astutil.Apply(callExpr, pre, post)

	return true
}
