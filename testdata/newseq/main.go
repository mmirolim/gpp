package main

import (
	"fmt"
	"strconv"
)

func main() {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	NewSeq_μ(fseq).
		Map(func(v float64) float64 { return v + 1 }).
		Filter(func(v float64) bool { return v < 300 }).
		Map(func(v float64) string { return strconv.Itoa(int(v)) }).
		Map(func(v string) styp { return styp{len(v)} }).
		Get(&out)
	fmt.Println("")
	fmt.Printf("Test NewSeq Map/Filter %+v\n", out)

	seq := []int{1, 2, 3, 4, 5, 6}
	var totalEvens int
	NewSeq_μ(seq).
		Filter(func(v int) bool { return v%2 == 0 }).
		Reduce(&totalEvens, func(acc, v int) int { return acc + v })
	fmt.Printf("Test NewSeq Reduce %+v\n", totalEvens)

}

func filter_μ(in, out, fn interface{}) {
	input := []_T{}
	res := &([]_T{})
	pred := (_PF)(nil)
	for i, v := range input {
		if pred(v) {
			*res = append(*res, input[i])
		}
	}
}

func map_μ(in, out, fn interface{}) {
	input := []_T{}
	res := &([]_T{})
	fun := (_MF)(nil)
	for i := range input {
		*res = append(*res, fun(input[i]))
	}
}

func reduce_μ(in, out, fn interface{}) {
	input := []_T{}
	accum := (*_T)(nil)
	fun := (_RF)(nil)
	for i := range input {
		*accum = fun(*accum, input[i])
	}
}

type seq_μ []_T

func NewSeq_μ(src interface{}) *seq_μ {
	seq0 := []_T{}
	return &seq_μ{seq0}
}

func (seq *seq_μ) Get(out interface{}) *seq_μ {
	output := &[]_T{}
	res := []_T{}
	for i := range res {
		*output = append(*output, res[i])
	}
	return seq
}

func (seq *seq_μ) Filter(fn interface{}) *seq_μ {
	f := (_PF)(nil)
	in := []_T{}
	out := &[]_T{}
	filter_μ(in, out, f)
	return seq
}

func (seq *seq_μ) Map(fn interface{}) *seq_μ {
	f := (_MF)(nil)
	in := []_T{}
	out := &([]_T{})
	map_μ(in, out, f)
	return seq
}

func (seq *seq_μ) Reduce(accum, fn interface{}) *seq_μ {
	out := accum
	f := (_RF)(nil)
	in := []_T{}
	reduce_μ(in, out, f)
	return seq
}

type _RF func(_T, _T) _T
type _PF func(_T) bool
type _MF func(_T) _T
type _T interface{}
type _G interface{}
