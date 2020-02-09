package main

import (
	"errors"
	"fmt"
)

func main() {
	var result int
	// failed on fPtrIntError
	err := try_μ(func() error {
		fname, _ := fStrError(false)
		// should return here
		_, result, _ = fPtrIntError(true)
		// should not reach here
		fErr(false)
		fmt.Printf("fname %+v\n", fname) // output for debug
		return nil
	})
	fmt.Println("")
	fmt.Printf("Expect (result, err) (0, fPtrIntError error), got (%d, %+v)\n", result, err)
	err = try_μ(func() error {
		_, _ = fStrError(false)
		_, result, _ = fPtrIntError(false)
		fErr(false)
		// should return here
		return nil
	})
	fmt.Printf("Expect (result, err) (1, <nil>), got (%d, %+v)\n", result, err)
}

func try_μ(fn interface{}) error {
	return nil
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
