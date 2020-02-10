package main

import (
	"errors"
	"fmt"
)

func main() {
	var result int
	// failed on fPtrIntError
	fmt.Println("")
	err := try_μ(func() error {
		log_μ("result before", result)
		result := 10
		log_μ("result after", result)
		_ = fErr(true)
		return nil
	})
	log_μ("try err", err)
}

func log_μ(args ...interface{}) {
}

func try_μ(fn interface{}) error {
	return nil
}

func fErr(toError bool) error {
	if toError {
		return errors.New("fErr error")
	}
	return nil
}
