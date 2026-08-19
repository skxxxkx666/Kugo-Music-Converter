package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunReleaseSelfTestIgnoresNormalArguments(t *testing.T) {
	if handled, exitCode := runReleaseSelfTest([]string{"--other"}); handled || exitCode != 0 {
		t.Fatalf("handled=%v exitCode=%d", handled, exitCode)
	}
}

func TestWriteReleaseSelfTestReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	report := releaseSelfTestReport{Success: true, Version: "v0.6.0", FFmpegReady: true}
	if err := writeReleaseSelfTestReport(path, report); err != nil {
		t.Fatalf("writeReleaseSelfTestReport() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded releaseSelfTestReport
	if err := json.Unmarshal(data, &decoded); err != nil || !decoded.Success || !decoded.FFmpegReady {
		t.Fatalf("decoded report = %#v, error = %v", decoded, err)
	}
}
