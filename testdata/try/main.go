package main

import (
	"errors"
	"fmt"

	mcr "github.com/mmirolim/gpp/macro"
	"gpp.com/try/lib"
)

func main() {
	slf := []func() error{func() error { return nil }}
	var result int
	err := mcr.Try_μ(func() error {
		fname, _ := fStrError(false)
		_, result, _ = fPtrIntError(false)
		slf[0]()
		NoErrReturn()
		if result == 1 {
			// should return here
			fErr(true)
		}
		// should not reach here
		fmt.Printf("fname %+v\n", fname) // output for debug
		err := errors.New("some custom error")
		return err
	})
	fmt.Println("")
	fmt.Printf("(result, err) = (%d, %+v)\n", result, err)
	var recs [][]string
	var bs []*lib.B
	err = mcr.Try_μ(func() error {
		_, _ = fStrError(false)
		_, result, _ = fPtrIntError(false)
		fErr(false)
		mcr.NewSeq_μ(recs).Map(lib.NewB).Ret(&bs)
		// should return here
		return nil
	})
	fmt.Printf("(result, err) = (%d, %+v)\n", result, err)
}

func NoErrReturn() string {
	return "return is not an error"
}

func fStrError(toError bool) (string, error) {
	if toError {
		return "", errors.New("fStrError error")
	}
	return "fStrError", nil
}

func fErr(toError bool) error {
	if toError {
		return errors.New("fErr error")
	}
	return nil
}

type A struct{}

func fPtrIntError(toError bool) (*A, int, error) {
	if toError {
		return nil, 0, errors.New("fPtrIntError error")
	}
	return &A{}, 1, nil
}
