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
		Map(func(v string) styp { return styp{len(v)} }).
		Get(&out)
	fmt.Println("")
	fmt.Printf("Test NewSeq Map/Filter %+v\n", out)

	seq := []int{1, 2, 3, 4, 5, 6}
	var totalEvens int
	macro.NewSeq_μ(seq).
		Filter(func(v int) bool { return v%2 == 0 }).
		Reduce(&totalEvens, func(acc, v int) int { return acc + v })
	fmt.Printf("Test NewSeq Reduce %+v\n", totalEvens)

}

func ftoa(v float64) string {
	return strconv.Itoa(int(v))
}
