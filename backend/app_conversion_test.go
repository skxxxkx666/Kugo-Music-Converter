package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"kugo-music-converter/internal/handler"
	"kugo-music-converter/internal/service"
)

type fakeDesktopConverter struct {
	started chan struct{}
	release chan struct{}
}

func (f *fakeDesktopConverter) ValidateLocalConversion(handler.LocalConversionRequest) error {
	return nil
}

func (f *fakeDesktopConverter) ConvertLocalPaths(
	ctx context.Context,
	_ handler.LocalConversionRequest,
	onEvent func(name string, payload any),
) (service.BatchSummary, error) {
	f.started <- struct{}{}
	onEvent("progress", service.BatchProgressEvent{File: "song.ncm", Percent: 10})

	result := service.BatchFileDoneEvent{
		File:    "song.ncm",
		Input:   `C:\Music\song.ncm`,
		Current: 1,
		Total:   1,
	}
	summary := service.BatchSummary{Total: 1, OutputDir: `C:\Music\Output`}
	select {
	case <-ctx.Done():
		result.Status = "error"
		result.Error = &service.BatchFileError{Code: handler.ErrCancelled, UserMessage: "转换已取消", Severity: "warning"}
		summary.Failed = 1
		summary.Cancelled = true
	case <-f.release:
		result.Status = "ok"
		result.Output = `C:\Music\Output\song.mp3`
		result.Percent = 100
		summary.Success = 1
	}
	summary.Results = []service.BatchFileDoneEvent{result}
	onEvent("file-done", result)
	return summary, nil
}

func (f *fakeDesktopConverter) RedetectDatabase() service.DBStatus {
	return service.DBStatus{}
}

func TestConversionTaskLifecycleAndCancellation(t *testing.T) {
	converter := &fakeDesktopConverter{
		started: make(chan struct{}, 2),
		release: make(chan struct{}, 1),
	}
	events := make(chan string, 16)
	app := NewApp("test")
	app.ctx = context.Background()
	app.converter = converter
	app.eventSink = func(name string, _ any) {
		events <- name
	}

	request := ConversionRequest{Paths: []string{`C:\Music\song.ncm`}, OutputDir: `C:\Music\Output`}
	firstTask, err := app.StartConversion(request)
	if err != nil {
		t.Fatalf("StartConversion() error = %v", err)
	}
	waitForSignal(t, converter.started, "first task start")
	if _, err := app.StartConversion(request); err == nil {
		t.Fatal("second StartConversion() error = nil while a task is active")
	}
	if app.CancelConversion("different-task") {
		t.Fatal("CancelConversion() accepted a different task id")
	}
	if !app.CancelConversion(firstTask) {
		t.Fatal("CancelConversion() rejected the active task")
	}
	waitForEvent(t, events, conversionCompleteEvent)

	secondTask, err := app.StartConversion(request)
	if err != nil {
		t.Fatalf("StartConversion() after completion error = %v", err)
	}
	if secondTask == firstTask {
		t.Fatal("task id was not advanced")
	}
	waitForSignal(t, converter.started, "second task start")
	converter.release <- struct{}{}
	waitForEvent(t, events, conversionCompleteEvent)
}

func TestStartConversionReturnsValidationError(t *testing.T) {
	app := NewApp("test")
	app.ctx = context.Background()
	app.converter = validationErrorConverter{}

	if _, err := app.StartConversion(ConversionRequest{}); err == nil {
		t.Fatal("StartConversion() error = nil, want validation error")
	}
}

type validationErrorConverter struct{}

func (validationErrorConverter) ValidateLocalConversion(handler.LocalConversionRequest) error {
	return errors.New("invalid request")
}

func (validationErrorConverter) ConvertLocalPaths(context.Context, handler.LocalConversionRequest, func(string, any)) (service.BatchSummary, error) {
	return service.BatchSummary{}, nil
}

func (validationErrorConverter) RedetectDatabase() service.DBStatus {
	return service.DBStatus{}
}

func waitForSignal(t *testing.T, channel <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForEvent(t *testing.T, events <-chan string, target string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			if event == target {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event %s", target)
		}
	}
}
