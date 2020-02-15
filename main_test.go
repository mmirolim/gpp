package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMacro(t *testing.T) {
	// Setup start
	testDir := filepath.Join(os.TempDir(), "gpp-test-macro")
	goPathTest := filepath.Join(testDir, "go")
	moduleName, err := getModuleName(".")
	if err != nil {
		t.Fatalf("getModuleName error %+v", err)
	}
	src := filepath.Join(goPathTest, "src", moduleName)
	// clean before running
	os.RemoveAll(src)
	err = os.MkdirAll(src, 0700)
	if err != nil {
		t.Fatalf("MkdirAll error %+v", err)
	}
	cmd := exec.Command("cp", "-r", ".", src)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("cp error %+v", err)
	}
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd error %+v", err)
	}
	err = os.Chdir(src)
	if err != nil {
		log.Fatalf("chdir %+v", err)
	}
	// Setup end
	defer func() {
		err = os.Chdir(curDir)
		if err != nil {
			log.Fatalf("chdir %+v", err)
		}
		if !t.Failed() {
			// let check directory on fail
			os.RemoveAll(src)
		}
	}()
	cases := []struct {
		desc   string
		srcDir string
		output string
		err    error
	}{
		{
			desc:   "Test NewSeq M/F/R fluent api",
			srcDir: filepath.Join(src, "testdata", "newseq"),
			output: `
NewSeq Map/Filter [{strLen:3} {strLen:4}]
NewSeq res [2] sum even 12 mult even 48
`,
			err: nil,
		},
		{
			desc:   "Test try_μ",
			srcDir: filepath.Join(src, "testdata", "try"),
			output: `
(result, err) = (1, fErr: fErr error)
(result, err) = (1, <nil>)
`,
			err: nil,
		},
		{
			desc:   "Test log_μ",
			srcDir: filepath.Join(src, "testdata", "log"),
			output: `
/main.go:15 result before result=0
/main.go:17 result after result=10
/main.go:20 try err err=<nil>
/lib/lib.go:8 LogLibFunc val=20
/main.go:21 log lib func result lib.LogLibFuncA=20
`,
			err: nil,
		},
	}

	var buf bytes.Buffer
	for i, tc := range cases {
		buf.Reset()
		err = parseDir(tc.srcDir, moduleName, nil)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}
		err = os.Chdir(tc.srcDir)
		if err != nil {
			log.Fatalf("chdir %+v", err)
		}

		cmd = exec.Command("go", "run", "main.go")
		cmd.Env = append(os.Environ(), "GOPATH="+goPathTest)
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err = cmd.Run()
		output := buf.String()
		if isUnexpectedErr(t, i, tc.desc, nil, err) {
			t.Errorf("cmd args %v\n%s", cmd.Args, output)
			continue
		}

		if output != tc.output {
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
