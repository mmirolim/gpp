package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Guard_μ checks errors and returns early from the enclosing function.
// Unlike Try_μ which wraps errors in a closure, Guard_μ injects
// if err != nil { return err } checks directly into the enclosing function body.
//
// Usage:
//
//	func myFunc() error {
//	    macro.Guard_μ(func() {
//	        f, _ := os.Open("file.txt")
//	        data, _ := io.ReadAll(f)
//	        fmt.Println(string(data))
//	    })
//	    return nil
//	}
//
// Expands to inline if err != nil { return ... } checks
// with early return from the enclosing function.
func Guard_μ(fn any) {}

const Guard_μSymbol = "Guard_μ"

// MacroGuardExpand is the expander for Guard_μ.
// It inlines error checks that return from the enclosing function,
// without wrapping in a closure.
func MacroGuardExpand(
	ctx *Context,
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if !checkIsMacroIdent(Guard_μSymbol, idents) {
		return false
	}
	if len(callArgs[0]) == 0 {
		return false
	}
	funcLit, ok := callArgs[0][0].(*ast.FuncLit)
	if !ok {
		fmt.Printf("WARN expected Guard macro, got %+v\n", callArgs[0])
		return false
	}

	// create new err variable
	errDecl, errIdent := createDeclStmt(token.VAR, "_guarderr_", &ast.Ident{Name: "error"})

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
				errFmt = fmt.Sprintf(`"guard %s: %%w"`, callName[:idx])
			}

			// Build: if _guarderr_ != nil { return fmt.Errorf("guard ...: %w", _guarderr_) }
			fmtCall := createCallExpr(
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: "fmt"},
					Sel: &ast.Ident{Name: "Errorf"},
				},
				[]ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: errFmt},
					errIdent,
				},
			)
			ifStmt := &ast.IfStmt{
				Cond: &ast.BinaryExpr{
					X: errIdent, Op: token.NEQ, Y: &ast.Ident{Name: "nil"},
				},
				Body: &ast.BlockStmt{List: []ast.Stmt{
					&ast.ReturnStmt{Results: []ast.Expr{fmtCall}},
				}},
			}
			bodyList = append(bodyList, ifStmt)
		}
		return bodyList
	}

	// The Guard_μ body gets inlined directly (not in a closure)
	// Replace the Guard_μ call with the expanded statements
	stmts := []ast.Stmt{errDecl}
	stmts = append(stmts, procRecur(funcLit.Body.List)...)

	// Wrap in a block and replace the original expression
	blockStmt := &ast.BlockStmt{List: stmts}
	cur.InsertAfter(blockStmt)
	cur.Delete()

	// Expand body macros
	astutil.Apply(blockStmt, pre, post)
	return true
}

func init() {
	MacroExpanders[Guard_μSymbol] = MacroGuardExpand
}
