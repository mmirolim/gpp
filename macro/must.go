package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Must_μ panics on error. Template for Must macro.
// Usage: macro.Must_μ(func() error { ... })
// Expands to an immediately-invoked func literal with panic-on-error checks.
func Must_μ(fn any) error {
	return nil
}

const Must_μSymbol = "Must_μ"

// MacroMustExpand is the expander for Must_μ.
// Like Try_μ but panics instead of returning errors.
func MacroMustExpand(
	ctx *Context,
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if !checkIsMacroIdent(Must_μSymbol, idents) {
		return false
	}
	if len(callArgs[0]) == 0 {
		return false
	}
	funcLit, ok := callArgs[0][0].(*ast.FuncLit)
	if !ok {
		fmt.Printf("WARN expected Must macro, got %+v\n", callArgs[0])
		return false
	}

	// create new err variable
	errDecl, errIdent := createDeclStmt(token.VAR, "_musterr_", &ast.Ident{Name: "error"})

	var procRecur func([]ast.Stmt) []ast.Stmt
	procRecur = func(stmts []ast.Stmt) []ast.Stmt {
		var bodyList []ast.Stmt
	OUTER:
		for _, stmt := range stmts {
			bodyList = append(bodyList, stmt)
			var cexp *ast.CallExpr
			var assignStmt *ast.AssignStmt

			switch rstmt := stmt.(type) {
			case *ast.AssignStmt:
				lastVar, ok := rstmt.Lhs[len(rstmt.Lhs)-1].(*ast.Ident)
				if !ok || lastVar.Name != "_" {
					continue OUTER
				}
				if cexp, ok = rstmt.Rhs[0].(*ast.CallExpr); !ok {
					continue OUTER
				}
				obj := resolveExpr(cexp.Fun, ctx.Pkg)
				funcDecl := obj.Decl.(*ast.FuncDecl)
				lastReturnType := funcDecl.Type.Results.List[len(funcDecl.Type.Results.List)-1].Type
				if typIdent, ok := lastReturnType.(*ast.Ident); !ok || typIdent.Name != "error" {
					continue OUTER
				}
				assignStmt = rstmt

			case *ast.ExprStmt:
				if cexp, ok = rstmt.X.(*ast.CallExpr); !ok {
					continue OUTER
				}
				obj := resolveExpr(cexp.Fun, ctx.Pkg)
				funcDecl := obj.Decl.(*ast.FuncDecl)
				if len(funcDecl.Type.Results.List) == 0 {
					continue OUTER
				}
				lastReturnType := funcDecl.Type.Results.List[len(funcDecl.Type.Results.List)-1].Type
				if typIdent, ok := lastReturnType.(*ast.Ident); !ok || typIdent.Name != "error" {
					continue OUTER
				}
				var lhs []ast.Expr
				for i := 0; i < len(funcDecl.Type.Results.List); i++ {
					lhs = append(lhs, &ast.Ident{Name: "_"})
				}
				rhs := []ast.Expr{cexp}
				assignStmt = createAssignStmt(lhs, rhs, token.ASSIGN)
				bodyList[len(bodyList)-1] = assignStmt

			default:
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
				}
				continue OUTER
			}

			// Replace last _ with err variable
			if len(assignStmt.Lhs) > 0 {
				assignStmt.Lhs[len(assignStmt.Lhs)-1] = errIdent
			} else {
				assignStmt.Lhs = []ast.Expr{errIdent}
			}

			// Build call name for error message
			callName, err := FormatNode(cexp)
			errFmt := `""`
			if err == nil {
				idx := strings.LastIndexByte(callName, '(')
				errFmt = fmt.Sprintf(`"must %s: %%w"`, callName[:idx])
			}

			// Build: if _musterr_ != nil { panic(fmt.Errorf("must ...: %w", _musterr_)) }
			panicCall := createCallExpr(
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: "fmt"},
					Sel: &ast.Ident{Name: "Errorf"},
				},
				[]ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: errFmt},
					errIdent,
				},
			)
			panicStmt := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun:  &ast.Ident{Name: "panic"},
					Args: []ast.Expr{panicCall},
				},
			}
			ifStmt := &ast.IfStmt{
				Cond: &ast.BinaryExpr{
					X: errIdent, Op: token.NEQ, Y: &ast.Ident{Name: "nil"},
				},
				Body: &ast.BlockStmt{List: []ast.Stmt{panicStmt}},
			}
			bodyList = append(bodyList, ifStmt)
		}
		return bodyList
	}

	// Add top level var err decl
	stmts := []ast.Stmt{errDecl}
	stmts = append(stmts, procRecur(funcLit.Body.List)...)

	funcLit.Body.List = stmts
	callExpr := createCallExpr(funcLit, nil)

	// The parent must be an ExprStmt (standalone call)
	if exprStmt, ok := cur.Node().(*ast.ExprStmt); ok {
		exprStmt.X = callExpr
	} else if assignStmt, ok := cur.Node().(*ast.AssignStmt); ok {
		// Must_μ used in assignment — shouldn't normally happen but handle gracefully
		_ = assignStmt
		cur.Replace(&ast.ExprStmt{X: callExpr})
	}

	// Expand body macros
	astutil.Apply(callExpr, pre, post)
	return true
}

func init() {
	MacroExpanders[Must_μSymbol] = MacroMustExpand
}
