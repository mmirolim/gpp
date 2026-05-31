package macro

import (
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

// Tap_μ executes a side-effect function in a pipeline without breaking the chain.
// Useful for logging, metrics, debugging, or tracing in the middle of a
// NewSeq_μ fluent pipeline.
//
//	macro.NewSeq_μ(items).
//	    Map(transform).
//	    Tap_μ(func(v ItemType) { log.Println("debug:", v) }).
//	    Filter(predicate).
//	    Ret(&result)
func Tap_μ(fn any) any { return nil }

const Tap_μSymbol = "Tap_μ"

// MacroTapExpand is the expander for Tap_μ.
// It inserts the tap function call as a side-effect statement in the pipeline
// and passes the sequence value through unchanged.
func MacroTapExpand(
	ctx *Context,
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	if !checkIsMacroIdent(Tap_μSymbol, idents) {
		return false
	}
	if len(callArgs) == 0 || len(callArgs[0]) == 0 {
		return false
	}

	// Tap_μ(fn) is used inside a chain like:
	//   seq.Tap_μ(func(v T) { ... })
	// The macro expander for NewSeq handles the pipeline wiring.
	// Tap just needs to be recognized as a valid pipeline stage.
	// The general expander will handle the body template.

	return false // let MacroGeneralExpand handle it via the template body
}

func init() {
	MacroExpanders[Tap_μSymbol] = MacroTapExpand
}
