package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// type for fluent M/F/R api
type seq_μ []_T

// Func signatures for macro templates
type _RF func(_T, _T, int) _T
type _PF func(_T, int) bool
type _MF func(_T, int) _T
type _T interface{}
type _G interface{}

// NewSeq_μ constructs new sequence and scope
// src must be slice and passed by value or pointer
func NewSeq_μ(src interface{}) *seq_μ {
	seq0 := []_T{}
	return &seq_μ{seq0}
}

// Ret copy computed values from seq to out
// out must be pointer to slice
func (seq *seq_μ) Ret(out interface{}) {
	output := &[]_T{}
	res := []_T{}
	for i := range res {
		*output = append(*output, res[i])
	}
}

// Filter values by fn predicate
// must be in form (val, index) func(_T [, int]) bool
// index is optional
func (seq *seq_μ) Filter(fn interface{}) *seq_μ {
	f := (_PF)(nil)
	in := []_T{}
	out := &[]_T{}
	Filter_μ(in, out, f)
	return seq
}

// Map apply fn func to seq to generate new seq
// must be in form func(_T [, int]) _T (any type)
// index is optional
func (seq *seq_μ) Map(fn interface{}) *seq_μ {
	f := (_MF)(nil)
	in := []_T{}
	out := &([]_T{})
	Map_μ(in, out, f)
	return seq
}

// Reduce apply fn func to seq and returns accum
// accum should be pointer type *_G
// fn type func(_G, _T [, int]) _G
// index is optional
func (seq *seq_μ) Reduce(accum, fn interface{}) *seq_μ {
	out := accum
	f := (_RF)(nil)
	in := []_T{}
	Reduce_μ(in, out, f)
	return seq
}

// Filter_μ (in, out) pointers to slices and fn func(_T [, int]) bool
func Filter_μ(in, out, fn interface{}) {
	input := []_T{}
	res := &([]_T{})
	pred := (_PF)(nil)
	for i, v := range input {
		if pred(v, i) {
			*res = append(*res, input[i])
		}
	}
}

// Map_μ (in, out) pointers to slices and fn func(_T [, int]) _G
func Map_μ(in, out, fn interface{}) {
	input := []_T{}
	res := &([]_T{})
	fun := (_MF)(nil)
	for i := range input {
		*res = append(*res, fun(input[i], i))
	}
}

// Reduce_μ in pointer/value to slice, out pointer *_G and fn func(_G, _T [, int]) _G
func Reduce_μ(in, out, fn interface{}) {
	input := []_T{}
	accum := (*_T)(nil)
	fun := (_RF)(nil)
	for i := range input {
		*accum = fun(*accum, input[i], i)
	}
}

// MacroNewSeq macro expander for sequence M/F/R
func MacroNewSeq(
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {

	var newSeqBlocks []ast.Stmt
	var lastNewSeqStmt ast.Stmt
	var lastNewSeqSeq *ast.Ident
	var blocks []ast.Stmt

	for i := 0; i < len(idents); i++ {
		ident := idents[i]
		// check if ident has return type
		var funDecl *ast.FuncDecl
		if ident.Obj == nil {
			name := fmt.Sprintf("%s.%s", Seq_μTypeSymbol, ident.Name)
			funDecl = MacroDecl[name]
			if funDecl == nil {
				fmt.Printf("WARN Method Decl not found %+v\n", name)
				continue
			}
		} else {
			var ok bool
			funDecl, ok = ident.Obj.Decl.(*ast.FuncDecl)
			if !ok {
				log.Fatalf("funDecl expected but got %+v", ident.Obj.Decl)
			}
		}
		if ident.Name != "NewSeq_μ" && newSeqBlocks != nil {
			// add to new block
			prevNewSeqBlock := newSeqBlocks[len(newSeqBlocks)-1]
			var prevObj *ast.Object
			switch val := prevNewSeqBlock.(type) {
			case *ast.AssignStmt:
				prevObj = val.Lhs[0].(*ast.Ident).Obj
			case *ast.DeclStmt:
				prevObj = lastNewSeqSeq.Obj
			}

			// assign ident to input
			callArgs[i] = append(callArgs[i], &ast.Ident{
				Name: fmt.Sprintf("%s%d", "seq", len(newSeqBlocks)-1),
				Obj:  prevObj,
			})
			var funcType *ast.FuncType
			if ident.Name != "Ret" {
				// handle func lit and functions
				fnID := 0
				if ident.Name == "Reduce" {
					fnID = 1
				}
				var funLit *ast.FuncLit
				switch fn := callArgs[i][fnID].(type) {
				case *ast.FuncLit:
					funcType = fn.Type
				default:
					obj := resolveExpr(fn, ApplyState.Pkg)
					if obj != nil && obj.Decl != nil {
						decl := obj.Decl.(*ast.FuncDecl)
						funcType = decl.Type
					} else {
						fmt.Printf("Currently unsupported Expr for MFR %T\n", fn)
						return false
					}
				}

				paramsNum := 0
				// count
				for _, field := range funcType.Params.List {
					paramsNum += len(field.Names) // same type arg
				}

				if paramsNum == 1 || (paramsNum == 2 && ident.Name == "Reduce") {
					if _, ok := callArgs[i][fnID].(*ast.FuncLit); !ok {
						// need to wrap ident into wrapper with correct args
						funLit = wrapExprToFuncLit(callArgs[i][fnID], funcType)
						funcType = funLit.Type
					}
					funcType.Params.List = append(funcType.Params.List,
						&ast.Field{
							Names: []*ast.Ident{
								{Name: "_"}, // ignored
							},
							Type: &ast.Ident{
								Name: "int",
							},
						})
					if funLit != nil {
						// swap call argument if we wrapped func
						callArgs[i][fnID] = funLit
					}
				}
			}
			if ident.Name != "Ret" && ident.Name != "Reduce" {
				var resultTyp ast.Expr
				if ident.Name == "Map" {
					resultTyp = funcType.Results.List[0].Type
				} else if ident.Name == "Filter" {
					resultTyp = funcType.Params.List[0].Type
				}
				arrType := &ast.ArrayType{
					Elt: resultTyp,
				}
				lastNewSeqStmt, lastNewSeqSeq = createDeclStmt(
					token.VAR, fmt.Sprintf("%s%d", "seq", len(newSeqBlocks)),
					arrType)
				newSeqBlocks = append(newSeqBlocks, lastNewSeqStmt)

				// assing unary op to output
				callArgs[i] = append(callArgs[i], &ast.UnaryExpr{
					Op: token.AND,
					X: &ast.Ident{
						Name: fmt.Sprintf("%s%d", "seq", len(newSeqBlocks)-1),
						Obj:  lastNewSeqSeq.Obj,
					},
				})
			}
		}
		body := copyBodyStmt(len(callArgs[i]),
			funDecl.Body, true)
		// find all body args defined as assignments
		var bodyArgs []*ast.AssignStmt
		for _, ln := range body.List {
			if st, ok := ln.(*ast.AssignStmt); ok {
				bodyArgs = append(bodyArgs, st)
			}
		}

		// TODO check that number of args is correct
		// switch Rhs with call args
		// TODO support multiple declaration in one line
		for i, carg := range callArgs[i] {
			bodyArgs[i].Rhs = []ast.Expr{carg}
		}
		// expand body macros
		astutil.Apply(body, pre, post)
		// New funcs which returns macro type should have parent scope
		if strings.HasPrefix(funDecl.Name.Name, "New") {
			newSeqBlocks = body.List
		} else {
			blocks = append(blocks, body)
		}

	}

	if len(blocks) > 0 {
		blockStmt := new(ast.BlockStmt)
		blockStmt.Lbrace = cur.Node().End()
		if newSeqBlocks != nil {
			blockStmt.List = append(blockStmt.List, newSeqBlocks...)
			newSeqBlocks = nil
			lastNewSeqStmt = nil
			lastNewSeqSeq = nil
		}
		blockStmt.List = append(blockStmt.List, blocks...)
		// insert as one block
		cur.InsertAfter(blockStmt)
		cur.Delete()
	}

	return true
}

// wrapExprToFuncLit contruct func literal to wrap func to modify it
// TODO results should be copied with changed
func wrapExprToFuncLit(fnExpr ast.Expr, fnType *ast.FuncType) *ast.FuncLit {
	fnLit := &ast.FuncLit{}
	fnLit.Type = &ast.FuncType{
		Results: fnType.Results,
	}
	var args []ast.Expr
	paramsList := make([]*ast.Field, len(fnType.Params.List))
	for i, param := range fnType.Params.List {
		paramsList[i] = param
		for i := range param.Names {
			args = append(args, param.Names[i])
		}
	}
	fnLit.Type.Params = &ast.FieldList{
		List: paramsList,
	}

	callFn := createCallExpr(fnExpr, args)
	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{callFn},
			},
		},
	}
	fnLit.Body = body
	return fnLit
}
