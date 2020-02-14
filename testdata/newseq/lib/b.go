package lib

import "github.com/mmirolim/gpp/macro"

type B struct{}

func (b *B) M() int {
	return 1
}

func (b B) MV() int {
	return 2
}

func (b B) Ident(v float64) float64 {
	return v
}

func FuncFromLib(v float64) float64 {
	return v
}

func Totals(seq []int) ([]int, int, int) {
	var totalEvens int
	totalProduct := 1
	var res []int
	macro.NewSeq_μ(seq).
		Filter(func(v int) bool { return v%2 == 0 }).
		Reduce(&totalEvens, func(acc, v, i int) int { return acc + v }).
		Reduce(&totalProduct, func(acc, v int) int { return acc * v }).
		Filter(func(v, i int) bool { return v == 2 }).
		Ret(&res)
	return res, totalEvens, totalProduct
}
