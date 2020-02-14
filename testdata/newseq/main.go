package main

import (
	"fmt"
	"strconv"

	"github.com/mmirolim/gpp/macro"
	"gpp.com/newseq/lib"
)

func main() {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	macro.NewSeq_μ(fseq).
		Map(func(v float64) float64 { return v + 1 }).
		Filter(func(v float64) bool { return v < 300 }).
		Map(ftoa).
		Map(func(v string, i int) styp { return styp{len(v) + i} }).
		Ret(&out)
	fmt.Println("")
	fmt.Printf("NewSeq Map/Filter %+v\n", out)

	res, totalEvens, totalProduct := lib.Totals([]int{1, 2, 3, 4, 5, 6})
	fmt.Printf("NewSeq res %d sum even %+v mult even %d\n", res, totalEvens, totalProduct)
}

func ftoa(v float64) string {
	return strconv.Itoa(int(v))
}
