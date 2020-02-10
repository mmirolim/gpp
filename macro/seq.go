package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

type seq_μ []_T

type _RF func(_T, _T) _T
type _PF func(_T) bool
type _MF func(_T) _T
type _T interface{}
type _G interface{}

func NewSeq_μ(src interface{}) *seq_μ {
	seq0 := []_T{}
	return &seq_μ{seq0}
}

func (seq *seq_μ) Get(out interface{}) *seq_μ {
	output := &[]_T{}
	res := []_T{}
	for i := range res {
		*output = append(*output, res[i])
	}
	return seq
}

func (seq *seq_μ) Filter(fn interface{}) *seq_μ {
	f := (_PF)(nil)
	in := []_T{}
	out := &[]_T{}
	Filter_μ(in, out, f)
	return seq
}

func (seq *seq_μ) Map(fn interface{}) *seq_μ {
	f := (_MF)(nil)
	in := []_T{}
	out := &([]_T{})
	Map_μ(in, out, f)
	return seq
}

func (seq *seq_μ) Reduce(accum, fn interface{}) *seq_μ {
	out := accum
	f := (_RF)(nil)
	in := []_T{}
	Reduce_μ(in, out, f)
	return seq
}

func Filter_μ(in, out, fn interface{}) {
	input := []_T{}
	res := &([]_T{})
	pred := (_PF)(nil)
	for i, v := range input {
		if pred(v) {
			*res = append(*res, input[i])
		}
	}
}

func Map_μ(in, out, fn interface{}) {
	input := []_T{}
	res := &([]_T{})
	fun := (_MF)(nil)
	for i := range input {
		*res = append(*res, fun(input[i]))
	}
}

func Reduce_μ(in, out, fn interface{}) {
	input := []_T{}
	accum := (*_T)(nil)
	fun := (_RF)(nil)
	for i := range input {
		*accum = fun(*accum, input[i])
	}
}

// special rules
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
		// TODO what to do if obj literal used? Prohibit from constructing
		// by unexported field?
		var funDecl *ast.FuncDecl
		if ident.Obj == nil {
			funDecl = MacroMethods[fmt.Sprintf("%s.%s", Seq_μTypeSymbol, ident.Name)]
			if funDecl == nil {
				fmt.Printf("WARN Method Decl not found %+v\n", ident.Name) // output for debug

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
			// TODO refactor what checks needed
			// create decl state for storing sequence
			funcLit, _ := callArgs[i][0].(*ast.FuncLit)
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
				Name: fmt.Sprintf("%s%d", "seq", i-1),
				Obj:  prevObj,
			})
			// TODO refactor
			if ident.Name != "Get" && ident.Name != "Reduce" {
				var resultTyp ast.Expr
				if ident.Name == "Map" {
					resultTyp = funcLit.Type.Results.List[0].Type
				} else if ident.Name == "Filter" {
					resultTyp = funcLit.Type.Params.List[0].Type
				}
				arrType := &ast.ArrayType{
					Elt: resultTyp,
				}
				lastNewSeqStmt, lastNewSeqSeq = newDeclStmt(
					token.VAR, fmt.Sprintf("%s%d", "seq", i),
					arrType)
				newSeqBlocks = append(newSeqBlocks, lastNewSeqStmt)

				// assing unary op to output
				callArgs[i] = append(callArgs[i], &ast.UnaryExpr{
					Op: token.AND,
					X: &ast.Ident{
						Name: fmt.Sprintf("%s%d", "seq", i),
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
