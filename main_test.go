package main

import (
	"bytes"
	"errors"
	"go/ast"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMacro(t *testing.T) {
	// Setup start
	testDir := filepath.Join(os.TempDir(), "gm-test-macro")
	// clean before running
	os.RemoveAll(testDir)
	err := os.MkdirAll(testDir, 0700)
	if err != nil {
		t.Fatalf("MkdirAll error %+v", err)
	}
	defer func() {
		if !t.Failed() {
			// let check directory on fail
			os.RemoveAll(testDir)
		}
	}()

	testDataDir := filepath.Join("./", "testdata")
	cmd := exec.Command("cp", "-r", testDataDir, testDir)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("cp error %+v", err)
	}
	testDir = filepath.Join(testDir, "testdata")
	// Setup end

	cases := []struct {
		desc    string
		testDir string
		output  string
		err     error
	}{
		{
			desc:    "Test NewSeq M/F/R fluent api",
			testDir: filepath.Join(testDir, "newseq"),
			output: `
Test NewSeq Map/Filter [{strLen:3} {strLen:3}]
Test NewSeq Reduce 12
`,
			err: nil,
		},
		{
			desc:    "Test try_μ",
			testDir: filepath.Join(testDir, "try"),
			output: `
Expect (result, err) (0, fPtrIntError error), got (0, fPtrIntError error)
Expect (result, err) (1, <nil>), got (1, <nil>)
`,
			err: nil,
		},
	}

	var buf bytes.Buffer
	for i, tc := range cases {
		buf.Reset()
		err = parseDir(tc.testDir)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		cmd = exec.Command("go", "run", filepath.Join(tc.testDir, "main.go"))
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err = cmd.Run()
		output := buf.String()
		if err != nil {
			log.Fatalf("go run error %+v\n%s", err, output)
		}
		if output != tc.output {
			t.Errorf("case [%d] %s\nexpected %s, got %s", i, tc.desc, tc.output, output)
		}
	}
}

func TestFnNameFromCallExpr(t *testing.T) {
	cases := []struct {
		callExpr *ast.CallExpr
		output   string
		err      error
	}{
		{
			&ast.CallExpr{
				Fun: &ast.Ident{
					Name: "F1",
				},
			}, "F1", nil},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "t",
					},
					Sel: &ast.Ident{
						Name: "Error",
					},
				},
			}, "t.Error", nil},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "pkg1",
					},
					Sel: &ast.Ident{
						Name: "F2",
					},
				},
			}, "pkg1.F2", nil,
		},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "pkgc",
					},
					Sel: &ast.Ident{
						Name: "Diff",
					},
				},
			}, "pkgc.Diff", nil,
		},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.SelectorExpr{
						X: &ast.Ident{
							NamePos: 95,
							Name:    "pk",
						},
						Sel: &ast.Ident{
							NamePos: 98,
							Name:    "Type",
						},
					},
					Sel: &ast.Ident{
						NamePos: 103,
						Name:    "Min",
					},
				}},
			"pk.Type.Min", nil,
		},
		{
			&ast.CallExpr{
				Fun: &ast.IndexExpr{
					X: &ast.Ident{
						NamePos: 307,
						Name:    "m",
						Obj: &ast.Object{
							Kind: 4,
							Name: "m",
							Data: int(0),
							Type: nil,
						},
					},
				},
			}, "", errors.New("unexpected value *ast.IndexExpr"),
		},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X: &ast.CallExpr{
								Fun: &ast.SelectorExpr{
									X: &ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X: &ast.CallExpr{
												Fun: &ast.Ident{
													Name: "NewFA",
												},
											},
											Sel: &ast.Ident{
												Name: "M1",
											},
										},
									},
									Sel: &ast.Ident{
										Name: "M1",
									},
								},
							},
							Sel: &ast.Ident{
								Name: "M2",
							},
						},
					},
					Sel: &ast.Ident{
						Name: "Name",
					},
				},
			}, "NewFA.M1.M1.M2.Name", nil,
		},
	}
	// TODO add cases with closures()().Method and arr[i].Param.Method calls
	for i, tc := range cases {
		fnName, err := fnNameFromCallExpr(tc.callExpr)
		if isUnexpectedErr(t, i, tc.output, tc.err, err) {
			continue
		}
		if fnName != tc.output {
			t.Errorf("case [%d]\nexpected %#v\ngot %#v", i, tc.output, fnName)
		}
	}
}

func isUnexpectedErr(t *testing.T, caseID int, desc string, expectedErr, goterr error) bool {
	t.Helper()
	var eStr, gotStr string
	if expectedErr != nil {
		eStr = expectedErr.Error()
	}
	if goterr != nil {
		gotStr = goterr.Error()
	}

	if eStr != gotStr {
		t.Errorf("case [%d] %s\nexpected error \"%s\"\ngot \"%s\"", caseID, desc, eStr, gotStr)
		return true
	}
	return false
}
