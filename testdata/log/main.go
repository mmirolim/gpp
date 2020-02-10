package main

import (
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
		return nil
	})
	log_μ("try err", err)
}

func log_μ(args ...interface{}) {
}

func try_μ(fn interface{}) error {
	return nil
}
