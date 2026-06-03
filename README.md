[![Go Report Card](https://goreportcard.com/badge/github.com/mmirolim/gpp)](https://goreportcard.com/badge/github.com/mmirolim/gpp)

# Gpp - go preprocessor (AST macro expander)

Gpp is a Go macro preprocessor and library. Expansion of all identified macros is done before calling `go build/run`. Macros defined in macro library have a signature of regular functions and defined as AST mutation function in Go code — **there is no new syntax to learn**.

Go 1.26 has excellent generics, but they still cannot have [method-level type parameters](https://github.com/golang/go/issues/77273) (proposal accepted, no target release). This means fluent/chained APIs like `.Map().Filter().Reduce()` are **impossible** in pure generics. Gpp macros expand these at compile time into efficient inline loops, producing zero-overhead, type-safe code — no reflection, no `unsafe`, no code generation bloat.

There are currently Try_μ, Guard_μ, Must_μ, Defer_μ, Log_μ, and Map/Filter/Reduce macros defined.

## Examples

 More examples in the testdata directory

### Try_μ — Error handling sugar

 Helps to omit manual and tedious error checking (`if err return err`), lets you focus on main code flow and guard the whole code blocks (inner blocks also checked) without polluting every line with checks.
 Errors wrapped with `fmt.Errorf` `%w` verb and can be investigated and handled after the try block.

 ```go
	err := macro.Try_μ(func() error {
		fname, _ := fStrError(false)
		_, result, _ = fPtrIntError(false)

		NoErrReturn() // does not return err, no need to check

		if result == 1 {
			fErr(true) // returns err
		}
		return nil
	})

	// handle particular errors after the block
	if errors.Is(err, SomeError) { someErrorHandler }
 ```

### Guard_μ — Inline early return

 Like Try_μ but inlines error checks directly into the enclosing function (no closure wrapper):

 ```go
	func processData() error {
		var val string
		macro.Guard_μ(func() {
			val, _ = readFile("data.txt") // returns from processData on error
		})
		fmt.Println(val)
		return nil
	}
 ```

### Must_μ — Panic on error

 For init/setup code where errors should be fatal:

 ```go
	macro.Must_μ(func() {
		config, _ := loadConfig("app.yaml") // panics with wrapped error
		db, _ := connectDB(config.DSN)
	})
 ```

### Defer_μ — Error-aware cleanup

 Handles cleanup errors that `defer f.Close()` silently ignores:

 ```go
	f, _ := os.Open("file.txt")
	macro.Defer_μ(f.Close) // logs error if Close() fails
 ```

 Expands to:

 ```go
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("gpp defer f.Close: %v", err)
		}
	}()
 ```

### Log_μ — Compile-time logging



 Logs can be selectively enabled/disabled on preprocessing stage — disabled logs become zero-cost no-ops.

 ```go
	num := 10
	macro.Log_μ(num, strr("hello"))
	macro.Log_μ("some context", lib.LogLibFuncA(20))
	macro.Log_μ("log anything")
 ```

 `gpp -run -log=main.go:1[0-9]` will enable logging only in main.go file on lines from 10-19.

### Map/Filter/Reduce

 Operations on any slice type — expand to loops on-call site without using `unsafe`, `interface{}` or reflection. Type safe with no significant performance loss.

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
 Other macros: MapKeys_μ, MapVals_μ, MapToSlice_μ, PrintMapKeys_μ, PrintMap_μ, PrintSlice_μ

### Tap_μ — Pipeline side-effects

 Execute a side-effect function in a NewSeq_μ pipeline without breaking the chain:

 ```go
	var debug []string
	macro.NewSeq_μ(data).
	    Tap_μ(func(v int) { debug = append(debug, fmt.Sprintf("item:%d", v)) }).
	    Filter(func(v int) bool { return v > 3 }).
	    Ret(&result)
 ```

 Supports `func(v T)` and `func(v T, i int)` signatures.

### `//gpp:derive String` — Auto-generate String()

 Generate a `String()` method for iota const types from a comment directive:

 ```go
	//gpp:derive String
	type Color int

	const (
	    Red Color = iota
	    Green
	    Blue
	)
 ```

 Generates a `func (c Color) String() string` with a switch statement over all constants. No macro import needed — works on any Go file.

### `gpp check` — Validate macro usage

 Check macro usage without building:

	 gpp -check -C myproject

 Reports unknown macros, wrong argument counts, and unsupported derive directives.

## Benchmarks

	goos: linux
	goarch: amd64
	BenchmarkNewSeqMacro-8             	 2759733	       432 ns/op
	BenchmarkNewSeqOpsHandWritten-8    	 3033134	       399 ns/op
	BenchmarkNewSeqOpsByReflection-8   	  118905	      9575 ns/op

 Macro-expanded code matches hand-written loops (8% overhead) and is **22x faster** than reflection.

## Installation

 Go 1.26+ required. `go` command must be available.

	go install github.com/mmirolim/gpp@latest

## Usage

 Run in the project directory:

	gpp -run          # build and run
	gpp -test         # run tests
	gpp -diff         # show macro expansion diff (for debugging)
	gpp -check        # validate macro usage without building
	gpp -run -log="main.go:1[0-9]"  # selective logging

	gpp -help
	Usage of gpp:
	  -C string
	        working directory (default ".")
	  -args string
	        args to go
	  -check
	        validate macro usage without building
	  -diff
	        show macro expansion diff without building
	  -log string
	        regex matching filename:line
	  -run
	        run built binary
	  -test
	        test binary

## Edge cases

- Macro functions should be used directly or assignment and usage should be in same local scope
- Needs more extensive testing

## AI-Assisted Development

 Gpp works well with AI coding agents. The `--diff` flag lets agents inspect macro expansions, and regular Go function signatures make macros easy for AI tools to understand and generate correctly. See `MODERNIZATION.md` for planned features targeting AI-assisted workflows.
