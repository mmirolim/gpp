package main

import (
	"fmt"
	"testing"

	"github.com/mmirolim/gpp/macro"
)

func TestNewSeqMacro(t *testing.T) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	macro.NewSeq_μ(fseq).Map(func(v float64) float64 { return v + 1 }).
		Filter(func(v float64) bool { return v < 300 }).
		Map(ftoa).
		Map(func(v string, i int) styp { return styp{len(v) + i} }).
		Ret(&out)
	fmt.Printf("%+v\n", out) // output for debug

	if len(out) != 2 {
		t.Errorf("expected 3, got %d", len(out))
	}
}
func TestNewSeqOpsHandWritten(t *testing.T) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp

	var incout []float64
	// Map
	for _, v := range fseq {
		incout = append(incout, v+1)
	}
	var filtered []float64
	// Filter
	for _, v := range incout {
		if v < 300 {
			filtered = append(filtered, v)
		}
	}
	// Map ftoa
	var strs []string
	for _, v := range filtered {
		strs = append(strs, ftoa(v))
	}
	for i, v := range strs {
		out = append(out, styp{len(v) + i})
	}
	fmt.Printf("%+v\n", out) // output for debug

	if len(out) != 2 {
		t.Errorf("expected 3, got %d", len(out))
	}
}

func BenchmarkNewSeqMacro(b *testing.B) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	for i := 0; i < b.N; i++ {
		macro.NewSeq_μ(fseq).Map(func(v float64) float64 { return v + 1 }).
			Filter(func(v float64) bool { return v < 300 }).
			Map(ftoa).
			Map(func(v string, i int) styp { return styp{len(v) + i} }).
			Ret(&out)
	}
	fmt.Printf("NewSeq %+v\n", len(out)) // output for debug
}

func BenchmarkNewSeqOpsHandWritten(b *testing.B) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	for i := 0; i < b.N; i++ {
		var incout []float64
		// Map
		for _, v := range fseq {
			incout = append(incout, v+1)
		}
		var filtered []float64
		// Filter
		for _, v := range incout {
			if v < 300 {
				filtered = append(filtered, v)
			}
		}
		// Map ftoa
		var strs []string
		for _, v := range filtered {
			strs = append(strs, ftoa(v))
		}
		for i, v := range strs {
			out = append(out, styp{len(v) + i})
		}
	}
	fmt.Printf("NewSeqManual %+v\n", len(out)) // output for debug
}
