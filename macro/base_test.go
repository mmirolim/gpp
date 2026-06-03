package macro

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

func TestIsMacroDecl(t *testing.T) {
	tests := []struct {
		name string
		decl *ast.FuncDecl
		want bool
	}{
		{
			name: "macro function",
			decl: &ast.FuncDecl{
				Name: ast.NewIdent("Try_μ"),
			},
			want: true,
		},
		{
			name: "regular function",
			decl: &ast.FuncDecl{
				Name: ast.NewIdent("handleError"),
			},
			want: false,
		},
		{
			name: "macro method on macro type",
			decl: &ast.FuncDecl{
				Recv: &ast.FieldList{List: []*ast.Field{{
					Type: &ast.Ident{Name: "seq_μ"},
				}}},
				Name: ast.NewIdent("Map"),
			},
			want: true,
		},
		{
			name: "method on regular type",
			decl: &ast.FuncDecl{
				Recv: &ast.FieldList{List: []*ast.Field{{
					Type: &ast.Ident{Name: "MyStruct"},
				}}},
				Name: ast.NewIdent("DoSomething"),
			},
			want: false,
		},
		{
			name: "nil decl",
			decl: nil,
			want: false,
		},
		{
			name: "macro method with pointer receiver",
			decl: &ast.FuncDecl{
				Recv: &ast.FieldList{List: []*ast.Field{{
					Type: &ast.StarExpr{X: &ast.Ident{Name: "seq_μ"}},
				}}},
				Name: ast.NewIdent("Filter"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMacroDecl(tt.decl); got != tt.want {
				t.Errorf("IsMacroDecl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckIsMacroIdent(t *testing.T) {
	tests := []struct {
		name    string
		macro   string
		idents  []*ast.Ident
		want    bool
	}{
		{
			name:  "direct call",
			macro: "Try_μ",
			idents: []*ast.Ident{
				{Name: "Try_μ"},
			},
			want: true,
		},
		{
			name:  "qualified call with macro lib",
			macro: "Try_μ",
			idents: []*ast.Ident{
				{Name: "macro"},
				{Name: "Try_μ"},
			},
			want: true,
		},
		{
			name:  "wrong macro name",
			macro: "Try_μ",
			idents: []*ast.Ident{
				{Name: "Log_μ"},
			},
			want: false,
		},
		{
			name:   "empty idents",
			macro:  "Try_μ",
			idents: []*ast.Ident{},
			want:   false,
		},
		{
			name:  "wrong lib prefix",
			macro: "Try_μ",
			idents: []*ast.Ident{
				{Name: "other"},
				{Name: "Try_μ"},
			},
			want: false, // "other" is not "macro" lib, and "other" != "Try_μ"
		},
		{
			name:  "aliased lib prefix",
			macro: "Try_μ",
			idents: []*ast.Ident{
				{Name: "mcr"},
				{Name: "Try_μ"},
			},
			want: false, // alias not handled by checkIsMacroIdent (resolved in pre())
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkIsMacroIdent(tt.macro, tt.idents); got != tt.want {
				t.Errorf("checkIsMacroIdent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCallExprAndParent(t *testing.T) {
	// Test with ExprStmt containing CallExpr
	callExpr := &ast.CallExpr{Fun: ast.NewIdent("foo")}
	exprStmt := &ast.ExprStmt{X: callExpr}
	parentStmt, result := getCallExprAndParent(exprStmt)
	if result == nil {
		t.Error("expected non-nil callExpr")
	}
	if parentStmt == nil {
		t.Error("expected non-nil parentStmt")
	}

	// Test with non-call ExprStmt
	exprStmt2 := &ast.ExprStmt{X: ast.NewIdent("x")}
	_, result2 := getCallExprAndParent(exprStmt2)
	if result2 != nil {
		t.Error("expected nil for non-call expr")
	}

	// Test with AssignStmt containing CallExpr
	assignStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("err")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent("bar")}},
	}
	parentStmt3, result3 := getCallExprAndParent(assignStmt)
	if result3 == nil {
		t.Error("expected non-nil callExpr in assignment")
	}
	if parentStmt3 == nil {
		t.Error("expected non-nil parentStmt for assignment")
	}
}

func TestCreateCallExpr(t *testing.T) {
	fun := ast.NewIdent("fn")
	args := []ast.Expr{ast.NewIdent("a"), ast.NewIdent("b")}
	expr := createCallExpr(fun, args)
	if expr == nil {
		t.Fatal("expected non-nil CallExpr")
	}
	if expr.Fun != fun {
		t.Error("expected Fun to match")
	}
	if len(expr.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(expr.Args))
	}
}

func TestCreateIfErrRetStmt(t *testing.T) {
	errIdent := ast.NewIdent("_tryerr_")
	retExpr := ast.NewIdent("someErr")
	stmt := createIfErrRetStmt(errIdent, retExpr)
	if stmt == nil {
		t.Fatal("expected non-nil IfStmt")
	}
	if stmt.Cond == nil {
		t.Error("expected non-nil condition")
	}
	if stmt.Body == nil || len(stmt.Body.List) != 1 {
		t.Error("expected body with one return statement")
	}
}

func TestCreateAssignStmt(t *testing.T) {
	lhs := []ast.Expr{ast.NewIdent("x")}
	rhs := []ast.Expr{ast.NewIdent("y")}
	stmt := createAssignStmt(lhs, rhs, token.DEFINE)
	if stmt == nil {
		t.Fatal("expected non-nil AssignStmt")
	}
	if stmt.Tok != token.DEFINE {
		t.Errorf("expected DEFINE token, got %v", stmt.Tok)
	}
}

func TestCreateDeclStmt(t *testing.T) {
	declStmt, ident := createDeclStmt(token.VAR, "myVar", &ast.Ident{Name: "int"})
	if declStmt == nil {
		t.Fatal("expected non-nil DeclStmt")
	}
	if ident == nil {
		t.Fatal("expected non-nil Ident")
	}
	if ident.Name != "myVar" {
		t.Errorf("expected ident name 'myVar', got %s", ident.Name)
	}
}

func TestAllMacroDecl(t *testing.T) {
	decls := make(map[string]*ast.FuncDecl)
	file := &ast.File{
		Name: ast.NewIdent("macro"),
		Decls: []ast.Decl{
			&ast.FuncDecl{Name: ast.NewIdent("Try_μ")},
			&ast.FuncDecl{Name: ast.NewIdent("Log_μ")},
			&ast.FuncDecl{Name: ast.NewIdent("regularFunc")},
			&ast.FuncDecl{
				Recv: &ast.FieldList{List: []*ast.Field{{
					Type: &ast.Ident{Name: "seq_μ"},
				}}},
				Name: ast.NewIdent("Map"),
			},
		},
	}

	AllMacroDecl(file, decls)

	// Should find Try_μ and Log_μ, but not regularFunc
	if _, ok := decls["Try_μ"]; !ok {
		t.Error("expected Try_μ in decls")
	}
	if _, ok := decls["Log_μ"]; !ok {
		t.Error("expected Log_μ in decls")
	}
	if _, ok := decls["regularFunc"]; ok {
		t.Error("did not expect regularFunc in decls")
	}
	// Should find seq_μ.Map
	if _, ok := decls["seq_μ.Map"]; !ok {
		t.Error("expected seq_μ.Map in decls")
	}
	if _, ok := decls["macro.Try_μ"]; !ok {
		t.Error("expected macro.Try_μ in decls")
	}
}

func TestGetMacroDeclByName(t *testing.T) {
	decls := map[string]*ast.FuncDecl{
		"Try_μ": {Name: ast.NewIdent("Try_μ")},
	}

	if d := getMacroDeclByName(decls, "Try_μ"); d == nil {
		t.Error("expected to find Try_μ")
	}
	if d := getMacroDeclByName(decls, "NotExist"); d != nil {
		t.Error("expected nil for unknown macro")
	}
}

func TestCopyBodyStmt(t *testing.T) {
	body := &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("x")},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{ast.NewIdent("1")},
			},
			&ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("y")},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{ast.NewIdent("2")},
			},
			&ast.ReturnStmt{Results: []ast.Expr{ast.NewIdent("nil")}},
		},
	}

	// With noreturns=true, return should be stripped
	result := copyBodyStmt(2, body, true)
	if len(result.List) != 2 {
		t.Errorf("expected 2 stmts (return stripped), got %d", len(result.List))
	}

	// With noreturns=false, return should be kept
	result2 := copyBodyStmt(2, body, false)
	if len(result2.List) != 3 {
		t.Errorf("expected 3 stmts (return kept), got %d", len(result2.List))
	}
}

func TestFormatNode(t *testing.T) {
	ident := ast.NewIdent("hello")
	str, err := FormatNode(ident)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if str != "hello" {
		t.Errorf("expected 'hello', got %q", str)
	}
}

func TestContext(t *testing.T) {
	ctx := &Context{
		MacroLibName: "macro",
		RemoveLib:    true,
		MacroDecls:   make(map[string]*ast.FuncDecl),
	}
	if ctx.MacroLibName != "macro" {
		t.Errorf("expected MacroLibName 'macro', got %s", ctx.MacroLibName)
	}
	if !ctx.RemoveLib {
		t.Error("expected RemoveLib to be true")
	}
}

func TestMacroSymbols(t *testing.T) {
	// Verify all macro symbols are registered in MacroExpanders
	expectedSymbols := []string{Try_μSymbol, Log_μSymbol, Guard_μSymbol, Must_μSymbol, Defer_μSymbol, Tap_μSymbol, Seq_μTypeSymbol}
	for _, sym := range expectedSymbols {
		if _, ok := MacroExpanders[sym]; !ok {
			t.Errorf("macro symbol %q not registered in MacroExpanders", sym)
		}
	}
}

func TestIdentsFromCallExpr(t *testing.T) {
	// Test simple function call: foo()
	call := &ast.CallExpr{Fun: ast.NewIdent("foo")}
	var idents []*ast.Ident
	var callArgs [][]ast.Expr
	IdentsFromCallExpr(call, &idents, &callArgs)
	if len(idents) != 1 || idents[0].Name != "foo" {
		t.Errorf("expected [foo], got %v", idents)
	}

	// Test selector call: pkg.Foo()
	call2 := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("macro"),
			Sel: ast.NewIdent("Try_μ"),
		},
	}
	idents = nil
	callArgs = nil
	IdentsFromCallExpr(call2, &idents, &callArgs)
	if len(idents) != 2 || idents[0].Name != "macro" || idents[1].Name != "Try_μ" {
		t.Errorf("expected [macro, Try_μ], got %v", idents)
	}
}

func TestGetFirstTypeInReturn(t *testing.T) {
	// Function returning *seq_μ — should extract "seq_μ"
	decl := &ast.FuncDecl{
		Type: &ast.FuncType{
			Results: &ast.FieldList{List: []*ast.Field{{
				Type: &ast.StarExpr{X: &ast.Ident{Name: "seq_μ"}},
			}}},
		},
	}
	result := getFirstTypeInReturn(decl)
	if result != "seq_μ" {
		t.Errorf("expected 'seq_μ', got %q", result)
	}

	// Function with no results
	decl2 := &ast.FuncDecl{Type: &ast.FuncType{}}
	result2 := getFirstTypeInReturn(decl2)
	if result2 != "" {
		t.Errorf("expected empty string, got %q", result2)
	}

	// nil decl
	result3 := getFirstTypeInReturn(nil)
	if result3 != "" {
		t.Errorf("expected empty string for nil, got %q", result3)
	}
}

func TestParseDeriveDirective(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single derive",
			input: "//gpp:derive String",
			want:  []string{"String"},
		},
		{
			name:  "multiple derives",
			input: "//gpp:derive String,Validate",
			want:  []string{"String", "Validate"},
		},
		{
			name:  "no derive",
			input: "// some other comment",
			want:  nil,
		},
		{
			name:  "derive with spaces",
			input: "//gpp:derive  String , Validate ",
			want:  []string{"String", "Validate"},
		},
		{
			name:  "empty derive",
			input: "//gpp:derive",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDeriveDirective(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseDeriveDirective(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseDeriveDirective(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindConstNamesForType(t *testing.T) {
	file := &ast.File{
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok: token.CONST,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{
							{Name: "Red"},
							{Name: "Green"},
							{Name: "Blue"},
						},
						Type: &ast.Ident{Name: "Color"},
					},
				},
			},
			&ast.GenDecl{
				Tok: token.CONST,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{{Name: "Active"}},
						Type:  &ast.Ident{Name: "Status"},
					},
				},
			},
		},
	}

	names := FindConstNamesForType(file, "Color")
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}
	expected := []string{"Red", "Green", "Blue"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, expected[i])
		}
	}

	names2 := FindConstNamesForType(file, "Status")
	if len(names2) != 1 || names2[0] != "Active" {
		t.Errorf("expected [Active], got %v", names2)
	}

	names3 := FindConstNamesForType(file, "Unknown")
	if len(names3) != 0 {
		t.Errorf("expected empty for unknown type, got %v", names3)
	}
}

func TestGenerateStringMethod(t *testing.T) {
	method := GenerateStringMethod("Color", []string{"Red", "Green", "Blue"})
	if method == nil {
		t.Fatal("expected non-nil FuncDecl")
	}
	if method.Name.Name != "String" {
		t.Errorf("expected method name 'String', got %q", method.Name.Name)
	}
	if method.Recv == nil || len(method.Recv.List) != 1 {
		t.Error("expected one receiver")
	}
	if len(method.Recv.List[0].Names) != 1 || method.Recv.List[0].Names[0].Name != "c" {
		t.Errorf("expected receiver name 'c', got %v", method.Recv.List[0].Names)
	}

	// Verify it formats correctly
	str, err := FormatNode(method)
	if err != nil {
		t.Fatalf("FormatNode error: %v", err)
	}
	if !strings.Contains(str, "case Red:") {
		t.Errorf("expected switch to contain 'case Red:', got %q", str)
	}
	if !strings.Contains(str, "Color(%d)") {
		t.Errorf("expected default case to contain 'Color(%%d)', got %q", str)
	}
}

func TestGenerateStringMethodShortName(t *testing.T) {
	// Test with a single-char type name
	method := GenerateStringMethod("T", []string{"A", "B"})
	if method == nil {
		t.Fatal("expected non-nil FuncDecl")
	}
	if method.Recv.List[0].Names[0].Name != "t" {
		t.Errorf("expected receiver 't' for type 'T', got %q", method.Recv.List[0].Names[0].Name)
	}
}
