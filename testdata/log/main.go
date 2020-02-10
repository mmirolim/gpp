package main

import (
	"fmt"

	"github.com/mmirolim/gpp/macro"
)

func main() {
	var result int
	// failed on fPtrIntError
	fmt.Println("")
	err := macro.Try_μ(func() error {
		macro.Log_μ("result before", result)
		result := 10
		macro.Log_μ("result after", result)
		return nil
	})
	macro.Log_μ("try err", err)
}
