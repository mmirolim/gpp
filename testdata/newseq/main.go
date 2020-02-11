package main

import (
	"fmt"
	"strconv"

	"github.com/mmirolim/gpp/macro"
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

	seq := []int{1, 2, 3, 4, 5, 6}
	var totalEvens int
	totalProduct := 1
	var res []int
	macro.NewSeq_μ(seq).
		Filter(func(v int) bool { return v%2 == 0 }).
		Reduce(&totalEvens, func(acc, v, i int) int { return acc + v }).
		Reduce(&totalProduct, func(acc, v int) int { return acc * v }).
		Filter(func(v, i int) bool { return v == 2 }).
		Ret(&res)

	fmt.Printf("NewSeq res %d sum even %+v mult even %d\n", res, totalEvens, totalProduct)

}

func ftoa(v float64) string {
	return strconv.Itoa(int(v))
}
