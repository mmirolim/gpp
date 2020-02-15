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
	err := macro.Try_μ(func() error {
		macro.Log_μ("result before", result)
		result := 10
		macro.Log_μ("result after", result)
		return nil
	})
	a := [][2]int{{1, 2}}
	macro.Log_μ("try err", err, a, a[0], sl(10)[0])
	macro.Log_μ("log lib func result", lib.LogLibFuncA(20))
}

func sl(i int) []float64 {
	return []float64{float64(i)}
}
