package main

import (
	"errors"
	"fmt"

	"github.com/mmirolim/gpp/macro"
)

func main() {
	fmt.Println("")

	// Test Defer_μ — no error case (cleanup succeeds)
	noErrDefer()

	// Test Defer_μ — error case (cleanup fails, logged)
	errDefer()
}

func noErrDefer() {
	f := &closer{err: nil}
	macro.Defer_μ(f.Close)
	fmt.Println("noErrDefer ok")
}

func errDefer() {
	f := &closer{err: errors.New("close failed")}
	macro.Defer_μ(f.Close)
	fmt.Println("errDefer ok")
}

type closer struct {
	err error
}

func (c *closer) Close() error {
	return c.err
}
