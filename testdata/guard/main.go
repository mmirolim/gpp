package main

import (
	"errors"
	"fmt"

	"github.com/mmirolim/gpp/macro"
)

func main() {
	fmt.Println("")

	// Test Must_μ — no error case
	macro.Must_μ(func() {
		v, _ := noErr(false)
		fmt.Println("must ok", v)
	})

	// Test Guard_μ — error case returns early
	guardErr()

	// Test Guard_μ — no error case
	guardOk()
}

func guardOk() error {
	var val string
	macro.Guard_μ(func() {
		val, _ = noErr(false)
	})
	fmt.Println("guard ok", val)
	return nil
}

func guardErr() error {
	macro.Guard_μ(func() {
		_, _ = noErr(true)
		fmt.Println("should not reach")
	})
	fmt.Println("should not reach either")
	return nil
}

func noErr(toError bool) (string, error) {
	if toError {
		return "", errors.New("noErr error")
	}
	return "ok", nil
}
