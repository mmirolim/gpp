package main

import (
	"fmt"

	"github.com/mmirolim/gpp/macro"
	"gpp.com/log/lib"
)

func main() {
	var result int
	// failed on fPtrIntError
	fmt.Println("")
	logger, try := macro.Log_μ, macro.Try_μ
	err := try(func() error {
		logger("result before", result)
		result := 10
		logger("result after", result)
		return nil
	})
	a := [][2]int{{1, 2}}
	logger("err, slice and index", err, a, a[0])
	macro.Log_μ("func calls", sl(10)[0], strr("hello"))

	logger("lib calls", lib.LogLibFuncA(20))
}

func sl(i int) []float64 {
	return []float64{float64(i)}
}

func strr(s string) string {
	return s
}
