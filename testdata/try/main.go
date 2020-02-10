package main

import (
	"errors"
	"fmt"

	"github.com/mmirolim/gpp/macro"
)

func main() {
	var result int
	// failed on fPtrIntError
	err := macro.Try_μ(func() error {
		fname, _ := fStrError(false)
		// should return here
		_, result, _ = fPtrIntError(true)
		// should not reach here
		fErr(false)
		fmt.Printf("fname %+v\n", fname) // output for debug
		return nil
	})
	fmt.Println("")
	fmt.Printf("(result, err) = (%d, %+v)\n", result, err)
	err = macro.Try_μ(func() error {
		_, _ = fStrError(false)
		_, result, _ = fPtrIntError(false)
		fErr(false)
		// should return here
		return nil
	})
	fmt.Printf("(result, err) = (%d, %+v)\n", result, err)
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
