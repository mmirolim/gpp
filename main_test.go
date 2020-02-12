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
NewSeq Map/Filter [{strLen:3} {strLen:4}]
NewSeq res [2] sum even 12 mult even 48
`,
			err: nil,
		},
		{
			desc:    "Test try_μ",
			testDir: filepath.Join(testDir, "try"),
			output: `
(result, err) = (1, fErr: fErr error)
(result, err) = (1, <nil>)
`,
			err: nil,
		},
		{
			desc:    "Test log_μ",
			testDir: filepath.Join(testDir, "log"),
			output: `
/tmp/gpp-test-macro/testdata/log/main.go:14 result before result=0
/tmp/gpp-test-macro/testdata/log/main.go:16 result after result=10
/tmp/gpp-test-macro/testdata/log/main.go:19 try err err=<nil>
`,
			err: nil,
		},
		// 		{
		// 			desc:    "Test Example application",
		// 			testDir: filepath.Join(testDir, "example"),
		// 			output: `
		// TODO DEFINE
		// `,
		// 			err: nil,
		// 		},
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
