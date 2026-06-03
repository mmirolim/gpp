package main

import (
	"fmt"

	"github.com/mmirolim/gpp/macro"
)

func main() {
	fmt.Println("")

	// Test Tap_μ in a pipeline — sees all elements before filter
	data := []int{1, 2, 3, 4, 5}
	var evens []int
	var tapLog []string

	macro.NewSeq_μ(data).
		Tap_μ(func(v int) { tapLog = append(tapLog, fmt.Sprintf("%d", v)) }).
		Filter(func(v int) bool { return v%2 == 0 }).
		Ret(&evens)

	fmt.Printf("Tap_μ evens %v\n", evens)
	fmt.Printf("Tap_μ log %v\n", tapLog)

	// Test Tap_μ with index parameter
	var idxLog []string
	var mapped []int

	macro.NewSeq_μ(data).
		Map(func(v int) int { return v * 10 }).
		Tap_μ(func(v int, i int) { idxLog = append(idxLog, fmt.Sprintf("%d:%d", i, v)) }).
		Ret(&mapped)

	fmt.Printf("Tap_μ mapped %v\n", mapped)
	fmt.Printf("Tap_μ idxLog %v\n", idxLog)
}
