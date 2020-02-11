package main

import (
	"fmt"
	"testing"

	"github.com/BurntSushi/ty/fun"
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

func TestNewSeqOpsByReflection(t *testing.T) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp

	incout := fun.Map(func(v float64) float64 { return v + 1 }, fseq).([]float64)
	filtered := fun.Filter(func(v float64) bool { return v < 300 }, incout).([]float64)
	strs := fun.Map(func(v float64) string { return ftoa(v) }, filtered).([]string)
	out = fun.Map(func(v string) styp { return styp{len(v)} }, strs).([]styp)

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
		out = out[:0]
		macro.NewSeq_μ(fseq).Map(func(v float64) float64 { return v + 1 }).
			Filter(func(v float64) bool { return v < 300 }).
			Map(ftoa).
			Map(func(v string, i int) styp { return styp{len(v) + i} }).
			Ret(&out)
	}
}

func BenchmarkNewSeqOpsHandWritten(b *testing.B) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	for i := 0; i < b.N; i++ {
		out = out[:0]
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
}

func BenchmarkNewSeqOpsByReflection(b *testing.B) {
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	for i := 0; i < b.N; i++ {
		incout := fun.Map(func(v float64) float64 { return v + 1 }, fseq).([]float64)
		filtered := fun.Filter(func(v float64) bool { return v < 300 }, incout).([]float64)
		strs := fun.Map(func(v float64) string { return ftoa(v) }, filtered).([]string)
		out = fun.Map(func(v string) styp { return styp{len(v)} }, strs).([]styp)
	}
	_ = out
}
