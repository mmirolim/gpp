# Gpp Modernization Plan

## Context & Analysis

### What Go 1.26 Generics Solve (making some macros obsolete)
1. **Map/Filter/Reduce on slices** → `slices` package + generic `Map[T,U]`, `Filter[T]`, `Reduce[T,A]` functions
2. **Type-safe collections** → generics eliminate `interface{}` need
3. **Result/Option types** → can be built with generics now

### What Generics STILL Can't Do (where macros remain valuable)
1. **Method-level type parameters** — Proposal accepted (#77273) but NO target release. This kills fluent/chained APIs like `seq.Map().Filter().Reduce()` in pure generics.
2. **AST-level code transformation** — Try_μ-style error handling rewrite is impossible with generics
3. **Compile-time selective logging** — Log_μ enables/disables at preprocessing time with zero runtime cost
4. **DSL creation** — Domain-specific language constructs via syntax transformation
5. **Zero-cost abstraction** — Macros expand to hand-written-quality code, no boxing/unboxing

---

## Phase 1: Modernization
**Status: ✅ COMPLETE**

### What
Update from Go 1.13 to Go 1.26, replace all deprecated APIs, remove obsolete tooling.

### Why
The codebase was frozen at Go 1.13 (2020). Deprecated APIs (`ioutil`, `interface{}`, Travis CI) make the project look abandoned and prevent using modern tooling. AI agents trained on modern Go would generate incompatible code.

### How
- Replace `ioutil.ReadFile`/`ioutil.WriteFile` → `os.ReadFile`/`os.WriteFile`
- Replace `interface{}` → `any` (Go 1.18 alias)
- Remove `github.com/kr/pretty` dependency → `fmt.Sprintf("%#v")`
- Update `golang.org/x/tools` to v0.45.0
- Delete `.travis.yml` (defunct service)
- Update all `go.mod` from `go 1.13` to `go 1.26`

### Verification
- [x] All existing tests pass unchanged
- [x] `go vet` clean
- [x] No deprecated API usage remaining

---

## Phase 2: Build Flow Refactoring
**Status: ✅ COMPLETE**

### What
Replace the `cp -r` + GOPATH hack with selective staging + proper Go module support.

### Why
The old build flow copied the **entire project** (including `.git`, binaries, data files) to a temp directory, set a custom `GOPATH`, and built there. This was:
- **Slow** — copying potentially GB of unnecessary files
- **Fragile** — GOPATH-based module resolution breaks with go.mod
- **Broken** — relative `replace` directives don't resolve from temp directories

### How
- Walk source tree, copy **only** `.go`, `go.mod`, `go.sum` files to staging
- Symlink `vendor/` instead of copying
- Resolve relative `replace` directives to absolute paths in staging go.mod
- Use `go build -o <output>` to write binary directly to cwd
- Remove GOPATH override entirely — let Go modules handle resolution
- Clean up staging dir after successful build

### Verification
- [x] 2x test speed improvement (2.6s vs 5.1s)
- [x] No GOPATH usage
- [x] Original source files never modified

---

## Phase 3: Clean Architecture
**Status: ✅ COMPLETE**

### What
Replace the global mutable `ApplyState` with a proper `Context` struct passed through closures.

### Why
The `ApplyState` global was a shared mutable struct written from multiple places. This:
- Made the processor unsafe for concurrent use
- Made unit testing impossible (tests would share state)
- Made the code hard to reason about (any function could mutate any field)

### How
- Define `macro.Context` struct with same fields as `ApplyState`
- Create `NewPre(ctx)` and `NewPost(ctx)` closures that capture the context
- Change `MacroExpander` signature to accept `ctx *Context` as first parameter
- Create per-file context in `parseDir` instead of mutating global
- Replace package-level `MacroDecl` var with local `macroDecls` map passed via context

### Verification
- [x] No references to `ApplyState` global remain (only a comment)
- [x] All macro expanders receive context explicitly
- [x] Unit tests added that test functions in isolation

---

## Phase 4: Enhanced Error Handling Macros
**Status: ✅ COMPLETE**

### What
Add `Guard_μ` (inline early return) and `Must_μ` (panic on error) macros.

### Why
Error handling remains the #1 verbosity complaint in Go surveys. `Try_μ` already helps but requires a closure wrapper. `Guard_μ` inlines directly into the enclosing function — zero overhead, natural flow. `Must_μ` handles the common init/setup pattern where errors should be fatal.

### How
- `Guard_μ`: Expands `func() { ... }` body into the enclosing function, replacing `_` with error variable and injecting `if err != nil { return fmt.Errorf("funcName: %w", err) }` after each error-returning call
- `Must_μ`: Same as Try_μ but generates `panic(fmt.Errorf(...))` instead of `return`
- Both register via `init()` in `MacroExpanders` map
- Both support nested blocks (for, if, range, switch, etc.)

### Verification
- [x] New test case in `testdata/guard/`
- [x] Integration test passes with `-race`
- [x] `gpp -diff -C testdata/guard` shows correct expansions

---

## Phase 5: Developer Experience
**Status: ✅ COMPLETE**

### What
Add `--diff` flag for debugging macro expansions, update README, add unit tests.

### Why
Without visibility into what macros expand to, developers (and AI agents) can't debug macro misuse. This is the #1 blocker for adoption — you need to see what your code becomes after preprocessing.

### How
- `--diff` flag: copies to staging, expands macros, walks both trees, prints colored line-by-line diff (red = removed, green = added)
- README: Conservative update — preserved original structure/examples, added new macro sections, updated installation to `go install`, removed Travis badges
- Unit tests: 26 tests in `macro/base_test.go` covering `IsMacroDecl`, `checkIsMacroIdent`, `getCallExprAndParent`, AST construction helpers, `AllMacroDecl`, `copyBodyStmt`, `FormatNode`, `IdentsFromCallExpr`, etc.

### Verification
- [x] `gpp -diff -C testdata/try` shows correct Try_μ expansion
- [x] 26 unit tests + 4 integration tests all pass
- [x] README preserves original examples

---

## Phase 6: Developer Features
**Status: 🔄 IN PROGRESS (6 of 10 done)**

### What
Practical macros and tooling improvements that make Go development more productive. These are general developer features, not AI-specific.

### Why
Go still requires verbose boilerplate for common patterns: error-aware cleanup, struct builders, validation, string methods. Macros can eliminate this boilerplate at compile time with zero runtime cost.

---

#### 6.1 `Defer_μ` — Error-aware cleanup
**Status: ✅ DONE**

- **What**: Wraps `defer f.Close()` with error handling — logs cleanup errors instead of silently ignoring them
- **Why**: `defer f.Close()` ignores the error. Every Go linter warns about this. Defer_μ makes it a one-liner.
- **How**: Expands `macro.Defer_μ(f.Close)` to `defer func() { if err := f.Close(); err != nil { log.Printf("gpp defer f.Close: %v", err) } }()`. Handles both method values (`f.Close`) and call expressions (`f.Close()`). Auto-adds `log` import.
- **Files**: `macro/defer.go`, `testdata/defer/`
- **Verification**: Integration test passes, `gpp -diff -C testdata/defer` shows correct expansion

---

#### 6.2 `//gpp:ignore` — Skip files from macro expansion
**Status: ✅ DONE**

- **What**: File-level directive to skip macro processing entirely
- **Why**: Generated code (protobufs, mock implementations, vendored stubs) should not be macro-expanded. Without this, gpp would try to process every `.go` file and potentially break generated code.
- **How**: Before stripping comments, scan for `//gpp:ignore` in file-level comments. If found, skip AST traversal for that file. Implemented in `hasIgnoreDirective()` in `main.go`.
- **Files**: `main.go` (added `hasIgnoreDirective` function)
- **Verification**: Unit testable by adding `//gpp:ignore` to any testdata file

---

#### 6.3 `Tap_μ` — Pipeline side-effects
**Status: ✅ DONE**

- **What**: Execute a side-effect function in a NewSeq_μ pipeline without breaking the chain
- **Why**: Developers need to add logging/metrics/debugging to data pipelines mid-chain. Go generics can't do this because methods can't introduce type params.
- **How**: Integrated directly into `MacroNewSeq` pipeline expander. Tap generates a for-range loop that calls the side-effect function but does NOT create a new pipeline variable — the next stage reuses the same sequence. Supports both `func(v T)` and `func(v T, i int)` signatures. Resolves param count from FuncLit or named functions via `resolveExpr`.
- **Files**: `macro/seq.go` (pipeline handling + Tap_μ method template + `_TF` type), `macro/tap.go`, `testdata/tap/`
- **Verification**: Integration test passes with both 1-param and 2-param tap functions. `gpp -diff -C testdata/tap` shows correct expansion.
```go
macro.NewSeq_μ(data).
    Map(transform).
    Tap_μ(func(v Item) { log.Println("debug:", v) }).
    Filter(predicate).
    Ret(&result)
```

---

#### 6.4 `//gpp:derive String` — Auto-generate String()
**Status: ✅ DONE** (String only; Validate, MarshalJSON not yet implemented)

- **What**: Comment-based auto-generation of `String()` method for iota const types
- **Why**: Every Go project with enum types needs a `String()` method. Writing switch statements by hand is tedious and error-prone.
- **How**: Parse `//gpp:derive String` comment directives on type declarations. Find const blocks using that type. Generate `func (t Type) String() string` with a switch statement and a default `fmt.Sprintf("Type(%d)", int(t))` case. Runs independently of macro imports — processes files before comment stripping in `parseDir`.
- **Files**: `macro/derive.go` (ParseDeriveDirective, FindConstNamesForType, GenerateStringMethod, ProcessDeriveDirectives), `main.go` (derive processing in parseDir), `testdata/derive/`
- **Verification**: Integration test passes with multiple iota types. `gpp -check -C testdata/derive` validates derive directives.
```go
//gpp:derive String
type Color int

const (
    Red Color = iota
    Green
    Blue
)
```

---

#### 6.5 `//gpp:builder`
**Status: ❌ Not started**

- **What**: Generate type-safe builder pattern from struct definitions
- **Why**: Builder pattern is verbose but Go-idiomatic. One of the most common code-generation tasks.
- **How**: Parse `//gpp:builder` on struct types. Generate `NewXxxBuilder(required...)`, `WithOptional()`, and `Build()` methods. Pointer fields = optional, value fields = required.

---

#### 6.6 `gpp check`
**Status: ✅ DONE**

- **What**: Validate macro usage without building. Returns structured diagnostics.
- **Why**: Waiting for `go build` to discover macro misuse is too slow. Need fast feedback.
- **How**: New `-check` flag loads packages and walks AST looking for `_μ`-suffixed calls. Checks: (1) is the macro name known? (2) does the argument count match? (3) are derive directives supported? Reports issues to stderr. Runs without expanding or building.
- **Files**: `main.go` (checkMacros function, -check flag), `macro/derive.go` (ParseDeriveDirective)
- **Verification**: `gpp -check -C testdata/try` reports "OK: no macro issues found." 

---

#### 6.7 `Defer_μ` with custom handler
**Status: ✅ DONE**

- **What**: Allow specifying a custom error handler instead of logging
- **Why**: Some projects want to track cleanup errors in metrics, not logs
- **How**: `macro.Defer_μ(f.Close, func(err error) { metrics.Count("close_err", 1) })`. Second optional argument replaces the default `log.Printf`. When custom handler is provided, the `log` import is NOT added.
- **Files**: `macro/defer.go`, `testdata/defer/main.go`
- **Verification**: Integration test passes — custom handler captures error correctly

---

#### 6.8 `gpp test --snapshot`
**Status: ❌ Not started**

- **What**: Snapshot testing for macro expansions — record golden files, diff on change
- **Why**: When macros change, developers need to see what changed in the expansion.
- **How**: On first run, write expanded files to `.gpp-snapshots/`. On subsequent runs, diff against snapshots. `--update` flag to accept changes.

---

#### 6.9 `gpp watch`
**Status: ❌ Not started**

- **What**: File watcher mode — auto-rebuild on change
- **Why**: Development workflow improvement for iterative coding sessions.
- **How**: Use `fsnotify` to watch `.go` files. Re-run preprocessing + build on change.

---

#### 6.10 Custom macro loading
**Status: ❌ Not started**

- **What**: Load macro expanders from external packages
- **Why**: Enables project-specific DSLs without forking gpp. Teams can define their own macros.
- **How**: Scan go.mod for packages with a `//gpp:macros` directive. Load their `MacroExpander` map via plugin or code generation.

---

## Phase 7: AI-Specific Features
**Status: ❌ NOT STARTED — NEEDS DESIGN**

### What
Features designed specifically for AI agents and LLM-assisted development workflows.

### Why
AI agents (Claude, Copilot, Cursor, etc.) interact with tools differently than humans. They need machine-readable output, fast feedback loops, and the ability to reason about macro expansions programmatically.

### Ideas (need analysis and design)

#### 7.1 `gpp check --json`
- **What**: Machine-readable diagnostics output
- **Why**: AI agents can parse JSON and auto-fix issues. Human-readable output requires parsing.
- **How**: Structured JSON with file, line, column, severity, message, suggested fix

#### 7.2 `gpp explain <macro>`
- **What**: Given a macro call, output a natural language description of what it expands to
- **Why**: AI agents need to decide whether to use a macro. Understanding the expansion helps them choose correctly.
- **How**: Parse the macro call, run the expander, format the expansion as readable Go code with a natural language summary

#### 7.3 `gpp suggest <file>`
- **What**: Analyze code and suggest where macros could simplify it
- **Why**: AI agents may not know all available macros. This helps them discover relevant macros for the code they're writing.
- **How**: Pattern matching on common boilerplate (manual error checks → Try_μ, verbose cleanup → Defer_μ, manual loops → NewSeq_μ)
