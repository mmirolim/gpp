package macro

import "fmt"

func MapKeys_μ(keys, m interface{}) {
	slKeys := &[]_T{}
	dic := map[_T]_G{}
	for k := range dic {
		*slKeys = append(*slKeys, k)
	}
}

func MapVals_μ(vals, m interface{}) {
	slVals := &[]_T{}
	dic := map[_T]_G{}
	for _, v := range dic {
		*slVals = append(*slVals, v)
	}
}

func PrintMap_μ(m interface{}) {
	arg2 := map[_T]_G{}
	PrintMapf_μ("%v : %v\n", arg2)
}

func PrintMapf_μ(f string, m interface{}) {
	arg1 := f
	arg2 := map[_T]_G{}
	for k, v := range arg2 {
		fmt.Printf(arg1, k, v) // output for debug
	}
}

func PrintMapKeys_μ(keys, m interface{}) {
	arg1 := []_T{}
	arg2 := map[_T]_G{}
	for i := range arg1 {
		fmt.Printf("%v : %v\n", arg1[i], arg2[arg1[i]]) // output for debug
	}
}
