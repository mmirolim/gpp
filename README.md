[![Build Status](https://travis-ci.org/mmirolim/gpp.svg)](https://travis-ci.org/mmirolim/gpp)
[![GoDoc](https://godoc.org/github.com/mmirolim/gpp?status.svg)](http://godoc.org/github.com/mmirolim/gpp)
[![codecov](https://codecov.io/gh/mmirolim/gpp/branch/master/graph/badge.svg)](https://codecov.io/gh/mmirolim/gpp)
[![Go Report Card](https://goreportcard.com/badge/github.com/mmirolim/gpp)](https://goreportcard.com/badge/github.com/mmirolim/gpp)

# Gpp - go preprocessor (AST hacking experiment)

Until everyone waits for Go 2.0 and Generics let’s sprinkle some sugar with “macros”. Gpp is macro preprocessor and library. Expansion of all identified macros is done before calling go build/run. Macros defined in macro library have a signature of regular functions and defined as AST mutation function in go code so there is no new syntax to learn.
There are currently Log_μ, Try_μ, and Map/Filter/Reduce macros defined. Benefits of AST macros are defining DSLs, simulate type parametric functions and add sugar to the language without losing compile-time type safety, code bloat with code generation and without using slow/inconvenient reflection or unsafe packages.
	
## Examples

 More examples in the testdata directory
 
 Try_μ macro helps to omit manual and tedious error checking (if err return err), let's you focus on main code flow and guard the whole code blocks (inner blocks also checked) without polluting every line with checks. 
 Errors wrapped with fmt.Errorf %w verb and can be investigated and handled after the try block.
 
 ```go
	// fails on fErr
	err := macro.Try_μ(func() error {
		fname, _ := fStrError(false)
		_, result, _ = fPtrIntError(false)
		
		NoErrReturn() // does not return err, no need to check
		
		if result == 1 {
			// should return here
			fErr(true) // returns err
		}
		// should not reach here
		fmt.Printf("fname %+v\n", fname) // output for debug
		return nil
	})
	
	// here you can handle particular errors 
	if errors.Is(err, SomeError) { someErrorHandler }
	if errors.Is(err, ErrPermission) { permissionErrHandler}
	
  ```	
  Try_μ macro expands to
  
  ```go
	err := func() error {
		var _tryerr_ error
		fname, _tryerr_ := fStrError(false)
		if _tryerr_ != nil {
			return fmt.Errorf("fStrError: %w", _tryerr_) // append callexpr and wrap error
		}
		_, result, _tryerr_ = fPtrIntError(false)
		if _tryerr_ != nil {
			return fmt.Errorf("fPtrIntError: %w", _tryerr_)
		}
		NoErrReturn()
		if result == 1 {
			_tryerr_ = fErr(true)
			if _tryerr_ != nil {
				return fmt.Errorf("fErr: %w", _tryerr_)
			}
		}
		_, _tryerr_ = fmt.Printf("fname %+v\n", fname)
		if _tryerr_ != nil {
			return fmt.Errorf("fmt.Printf: %w", _tryerr_)
		}
		return _tryerr_
	}()
	
  ```
	
	
  Log_μ to log without paying the cost of runtime calls and indirections.Logs can be selectively enabled/disabled on preprocessing stage no need to manually guard logs call/remove them. 
  gpp -run -log=main.go:1[0-9] will enable logging only in main.go file on lines from 10-19.
  
  ```go
	num := 10
	macro.Log_μ(num, strr("hello"))
	macro.Log_μ("some context", lib.LogLibFuncA(20))
	macro.Log_μ("log anything")
  ```
  
  Expands
  
  ```go
	num := 10
	fmt.Printf("/log/main.go:18 num=%#v strr('hello')=%#v\n", num, strr("hello"))
	fmt.Printf("/log/main.go:19 %v lib.LogLibFuncA(20)=%#v\n", "some context", lib.LogLibFuncA(20))	
	__nooplog_("log anything") // no op stub if line is disabled
  ```


 Map/Filter/Reduce operations on any slice type, they expand to loops and block statement on-call site without using unsafe, interface{} or reflection so it is type safe and there is no significant performance loss. Map arg func(T [, int]) G, Filter arg func(T [, int]) bool and Reduce args T, func(T, G [, int]) T
  
  ```go
	fseq := []float64{100, 200, 300, 400, 500, 600}
	type styp struct{ strLen int }
	var out []styp
	macro.NewSeq_μ(fseq).
		Map(func(v float64) float64 { return v + 1 }).
		Filter(func(v float64) bool { return v < 300 }).
		Map(ftoa).
		Map(func(v string, i int) styp { return styp{len(v) + i} }).
		Ret(&out)
	
	seq := []int{1, 2, 3, 4, 5, 6}
	var sumOfEvens int
	macro.NewSeq_μ(seq).
		Filter(func(v int) bool { return v%2 == 0 }).
		Reduce(&sumOfEvens, func(acc, v, i int) int { return acc + v }).
  ```
  Other macros MapKeys_μ, MapVals_μ, MapToSlice_μ, PrintMapKeys_μ, PrintMap_μ, PrintSlice_μ
  
## Edge cases

- Early prototype
- Macro functions should be used directly or assignment and usage should be in same local scope
- gpp copy all files to temp directory to parse, rewrite and build it, build may fail if dependencies not found and/or may take long time to load. Enabling go mod and vendoring may help to fix some issues.
- Needs more extensive testing

## Benchmarks

  ```go
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
  ```

	goos: linux
	goarch: amd64
	BenchmarkNewSeqMacro-8             	 2759733	       432 ns/op
	BenchmarkNewSeqOpsHandWritten-8    	 3033134	       399 ns/op
	BenchmarkNewSeqOpsByReflection-8   	  118905	      9575 ns/op
 
## Installation
	
 gpp requires to go command to be available
	
	go get -u github.com/mmirolim/gpp

	
## Usage
	
 Run in the project directory

	gpp -run


	gpp -help
	Usage of gpp:
	-C string
		  working directory (default ".")
	-args string
		  args to go
	-run
		  run run binary
	-test
		  test binary


