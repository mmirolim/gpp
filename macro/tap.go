package macro

import (
	"fmt"
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
//
// Tap_μ is handled directly by MacroNewSeq as a pipeline stage — it generates
// a for-range loop that calls the tap function for each element but does not
// create a new pipeline variable. The next stage reuses the same sequence.
//
// Supports both func(v T) and func(v T, i int) signatures.
func Tap_μ(fn any) any { return nil }

const Tap_μSymbol = "Tap_μ"

// MacroTapExpand is the fallback expander for standalone Tap_μ usage.
// In practice, Tap_μ is always used inside a NewSeq_μ chain and is
// handled by MacroNewSeq's pipeline processing.
func MacroTapExpand(
	ctx *Context,
	cur *astutil.Cursor,
	parentStmt ast.Stmt,
	idents []*ast.Ident,
	callArgs [][]ast.Expr,
	pre, post astutil.ApplyFunc) bool {
	// Tap_μ in a NewSeq chain is handled by MacroNewSeq.
	// If we reach here, it means Tap_μ was used outside a chain (unsupported).
	if checkIsMacroIdent(Tap_μSymbol, idents) {
		fmt.Printf("WARN Tap_μ must be used inside a NewSeq_μ chain\n")
	}
	return false
}

func init() {
	MacroExpanders[Tap_μSymbol] = MacroTapExpand
}
