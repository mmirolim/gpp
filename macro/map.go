package macro

import "fmt"

// Convenience macros

// MapKeys_μ returns map keys
func MapKeys_μ(keys, m interface{}) {
	slKeys := &[]_T{}
	dic := map[_T]_G{}
	for k := range dic {
		*slKeys = append(*slKeys, k)
	}
}

// MapVals_μ returns map values
func MapVals_μ(vals, m interface{}) {
	slVals := &[]_T{}
	dic := map[_T]_G{}
	for _, v := range dic {
		*slVals = append(*slVals, v)
	}
}

// MapToSlice_μ apply f to elements of m to generate sl
func MapToSlice_μ(sl, m, f interface{}) {
	slice := &[]interface{}{}
	dic := map[_T]_G{}
	proc := (func(_T, _G) _T)(nil)
	for k, v := range dic {
		*slice = append(*slice, proc(k, v))
	}
}

// PrintMap_μ prints map
func PrintMap_μ(m interface{}) {
	arg2 := map[_T]_G{}
	PrintMapf_μ("%v : %v\n", arg2)
}

// PrintMapf_μ prints map in f format
func PrintMapf_μ(f string, m interface{}) {
	arg1 := f
	arg2 := map[_T]_G{}
	for k, v := range arg2 {
		fmt.Printf(arg1, k, v)
	}
}

// PrintMapKeys_μ prints provided keys and values
func PrintMapKeys_μ(keys, m interface{}) {
	arg1 := []_T{}
	arg2 := map[_T]_G{}
	for i := range arg1 {
		fmt.Printf("%v : %v\n", arg1[i], arg2[arg1[i]])
	}
}
