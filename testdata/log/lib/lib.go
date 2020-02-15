package lib

import (
	m "github.com/mmirolim/gpp/macro"
)

func LogLibFuncA(val int) int {
	m.Log_μ("LogLibFunc", val)
	return val
}
