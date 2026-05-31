# Gpp - Go Preprocessor

## Project Overview
Gpp is a Go AST macro preprocessor. It parses Go source files, identifies macro calls (suffixed with `_μ`), expands them via AST transformations, and then builds/runs the resulting code.

## Architecture
- `main.go` - CLI entry: flag parsing, project copy-to-tmp, AST processing pipeline, build/run
- `macro/base.go` - Core infrastructure: ApplyState, Pre/Post AST visitors, macro registry, utilities
- `macro/try.go` - Try_μ: automatic error checking/wrapping inside func literals
- `macro/log.go` - Log_μ: compile-time logging with line-level enable/disable
- `macro/seq.go` - NewSeq_μ: type-safe Map/Filter/Reduce fluent API via AST expansion

## Key Design Decisions
- Macros are regular Go function signatures (no new syntax)
- Macro bodies serve as templates; AST expansion replaces template vars with call-site args
- The `_μ` suffix identifies macro functions
- Global mutable `ApplyState` holds processing context per file

## Modernization Plan (Phases)
See MODERNIZATION.md for the full phased plan.

## Development
- Go 1.26+ required
- Tests in `main_test.go` exercise all macros via testdata/ projects
- Run tests: `go test -v ./...`
