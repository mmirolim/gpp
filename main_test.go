package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacro(t *testing.T) {
	// Resolve project root (where go.mod lives)
	projectRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd error %+v", err)
	}
	moduleName, err := getModuleName(projectRoot)
	if err != nil {
		t.Fatalf("getModuleName error %+v", err)
	}

	// Create a shared staging copy of the project for all test cases
	stagingBase, err := os.MkdirTemp("", "gpp-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp error %+v", err)
	}
	defer func() {
		if !t.Failed() {
			os.RemoveAll(stagingBase)
		}
	}()

	// Copy the project module files to staging
	if err := copyModuleToStaging(projectRoot, stagingBase); err != nil {
		t.Fatalf("copyModuleToStaging error %+v", err)
	}

	cases := []struct {
		desc       string
		srcDir     string
		output     string
		err        error
		looseMatch bool // if true, check output contains expected string instead of exact match
	}{
		{
			desc:   "Test NewSeq M/F/R fluent api",
			srcDir: filepath.Join(stagingBase, "testdata", "newseq"),
			output: `
NewSeq Map/Filter [{strLen:3} {strLen:4}]
NewSeq res [2] sum even 12 mult even 48
`,
			err: nil,
		},
		{
			desc:   "Test try_μ",
			srcDir: filepath.Join(stagingBase, "testdata", "try"),
			output: `
(result, err) = (1, fErr: fErr error)
(result, err) = (1, <nil>)
`,
			err: nil,
		},
		{
			desc:   "Test log_μ",
			srcDir: filepath.Join(stagingBase, "testdata", "log"),
			output: `
/main.go:16 result before result=0
/main.go:18 result after result=10
/main.go:22 err, slice and index err=<nil> a=[][2]int{[2]int{1, 2}} a[0]=[2]int{1, 2}
/main.go:23 func calls sl(10)[0]=10 strr('hello')="hello"
/lib/lib.go:8 LogLibFunc val=20
/main.go:25 lib calls lib.LogLibFuncA(20)=20
`,
			err: nil,
		},
		{
			desc:   "Test guard_μ and must_μ",
			srcDir: filepath.Join(stagingBase, "testdata", "guard"),
			output: `
must ok ok
guard ok ok
`,
			err: nil,
		},
		{
			desc:   "Test defer_μ",
			srcDir: filepath.Join(stagingBase, "testdata", "defer"),
			output: `
noErrDefer ok
customHandler custom err
errDefer ok
`,
			err:        nil,
			looseMatch: true, // error case also logs with dynamic timestamp
		},
		{
			desc:   "Test tap_μ",
			srcDir: filepath.Join(stagingBase, "testdata", "tap"),
			output: `
Tap_μ evens [2 4]
Tap_μ log [1 2 3 4 5]
Tap_μ mapped [10 20 30 40 50]
Tap_μ idxLog [0:10 1:20 2:30 3:40 4:50]
`,
			err: nil,
		},
		{
			desc:   "Test gpp:derive String",
			srcDir: filepath.Join(stagingBase, "testdata", "derive"),
			output: `
Red=Red Blue=Blue Unknown=Color(99)
Active=StatusActive Inactive=StatusInactive Pending=StatusPending
StatusUnknown=Status(0)
`,
			err: nil,
		},
	}

	var buf bytes.Buffer
	for i, tc := range cases {
		buf.Reset()

		// Parse and expand macros in the testdata subdirectory
		err := parseDir(tc.srcDir, moduleName, nil)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		// Build the preprocessed code
		cmd := exec.Command("go", "build", "-o", filepath.Join(tc.srcDir, "main"), "main.go")
		cmd.Dir = tc.srcDir
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err = cmd.Run()
		output := buf.String()
		if isUnexpectedErr(t, i, tc.desc, nil, err) {
			t.Errorf("cmd args %v\n%s", cmd.Args, output)
			continue
		}

		// Run the built binary
		buf.Reset()
		cmd = exec.Command(filepath.Join(tc.srcDir, "main"))
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err = cmd.Run()
		output = buf.String()
		if isUnexpectedErr(t, i, tc.desc, nil, err) {
			t.Errorf("cmd args %v\n%s", cmd.Args, output)
			continue
		}
		if tc.looseMatch {
			if !strings.Contains(output, tc.output) {
				t.Errorf("case [%d] %s\nexpected output to contain %q\n got %q", i, tc.desc, tc.output, output)
			}
		} else if output != tc.output {
			t.Errorf("case [%d] %s\nexpected %s, got %s", i, tc.desc, tc.output, output)
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
