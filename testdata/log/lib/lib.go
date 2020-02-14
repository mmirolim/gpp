package lib

import (
	"github.com/mmirolim/gpp/macro"
)

func LogLibFuncA(val int) int {
	macro.Log_μ("LogLibFunc", val)
	return val
}
