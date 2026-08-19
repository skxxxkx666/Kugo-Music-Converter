package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kugo-music-converter/internal/handler"
	"kugo-music-converter/internal/service"
)

const (
	conversionProgressEvent = "conversion:progress"
	conversionFileDoneEvent = "conversion:file-done"
	conversionCompleteEvent = "conversion:complete"
	conversionErrorEvent    = "conversion:error"
)

type ConversionRequest struct {
	Paths        []string `json:"paths"`
	OutputDir    string   `json:"outputDir"`
	DBPath       string   `json:"dbPath,omitempty"`
	OutputFormat string   `json:"outputFormat"`
	MP3Quality   int      `json:"mp3Quality"`
	Concurrency  int      `json:"concurrency"`
}

type desktopConverter interface {
	ValidateLocalConversion(handler.LocalConversionRequest) error
	ConvertLocalPaths(context.Context, handler.LocalConversionRequest, func(name string, payload any)) (service.BatchSummary, error)
	RedetectDatabase() service.DBStatus
}

type ConversionProgressMessage struct {
	TaskID   string                     `json:"taskId"`
	Progress service.BatchProgressEvent `json:"progress"`
}

type ConversionFileDoneMessage struct {
	TaskID string                     `json:"taskId"`
	Result service.BatchFileDoneEvent `json:"result"`
}

type ConversionCompleteMessage struct {
	TaskID  string               `json:"taskId"`
	Summary service.BatchSummary `json:"summary"`
}

type ConversionErrorMessage struct {
	TaskID      string `json:"taskId"`
	Code        string `json:"code"`
	UserMessage string `json:"userMessage"`
	Suggestion  string `json:"suggestion,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

func (a *App) StartConversion(request ConversionRequest) (string, error) {
	a.mu.Lock()
	if a.ctx == nil {
		a.mu.Unlock()
		return "", errors.New("桌面窗口尚未就绪")
	}
	if a.activeTaskID != "" {
		a.mu.Unlock()
		return "", errors.New("已有转换任务正在进行")
	}
	converter := a.converter
	a.mu.Unlock()

	if converter == nil {
		return "", errors.New("FFmpeg 运行时尚未就绪")
	}
	localRequest := handler.LocalConversionRequest{
		Paths:        request.Paths,
		OutputDir:    request.OutputDir,
		DBPath:       request.DBPath,
		OutputFormat: request.OutputFormat,
		MP3Quality:   request.MP3Quality,
		Concurrency:  request.Concurrency,
	}
	if err := converter.ValidateLocalConversion(localRequest); err != nil {
		return "", err
	}

	a.mu.Lock()
	if a.activeTaskID != "" {
		a.mu.Unlock()
		return "", errors.New("已有转换任务正在进行")
	}
	a.taskSequence++
	taskID := fmt.Sprintf("conversion-%d", a.taskSequence)
	taskContext, cancel := context.WithCancel(a.ctx)
	a.activeTaskID = taskID
	a.activeCancel = cancel
	a.mu.Unlock()

	go a.runConversion(taskContext, taskID, converter, localRequest)
	a.nativeTaskStart()
	return taskID, nil
}

func (a *App) CancelConversion(taskID string) bool {
	a.mu.RLock()
	activeTaskID := a.activeTaskID
	cancel := a.activeCancel
	a.mu.RUnlock()

	if activeTaskID == "" || cancel == nil || strings.TrimSpace(taskID) != activeTaskID {
		return false
	}
	cancel()
	return true
}

func (a *App) runConversion(
	ctx context.Context,
	taskID string,
	converter desktopConverter,
	request handler.LocalConversionRequest,
) {
	summary, err := converter.ConvertLocalPaths(ctx, request, func(name string, payload any) {
		switch event := payload.(type) {
		case service.BatchProgressEvent:
			a.nativeTaskProgress(event.Percent)
			a.emitConversionEvent(conversionProgressEvent, ConversionProgressMessage{TaskID: taskID, Progress: event})
		case service.BatchFileDoneEvent:
			a.registerPreviewResult(event)
			a.emitConversionEvent(conversionFileDoneEvent, ConversionFileDoneMessage{TaskID: taskID, Result: event})
		}
	})
	if err != nil {
		a.finishConversion(taskID)
		a.nativeTaskFailed()
		a.emitConversionEvent(conversionErrorEvent, mapConversionError(taskID, err))
		return
	}
	for _, result := range summary.Results {
		a.registerPreviewResult(result)
	}

	a.finishConversion(taskID)
	a.nativeTaskComplete(summary.Cancelled, summary.Success, summary.Failed)
	a.emitConversionEvent(conversionCompleteEvent, ConversionCompleteMessage{TaskID: taskID, Summary: summary})
}

func (a *App) finishConversion(taskID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeTaskID != taskID {
		return
	}
	if a.activeCancel != nil {
		a.activeCancel()
	}
	a.activeTaskID = ""
	a.activeCancel = nil
}

func (a *App) emitConversionEvent(name string, payload any) {
	if a.eventSink != nil {
		a.eventSink(name, payload)
		return
	}
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, name, payload)
}

func mapConversionError(taskID string, err error) ConversionErrorMessage {
	var appErr *handler.AppError
	if errors.As(err, &appErr) {
		return ConversionErrorMessage{
			TaskID:      taskID,
			Code:        appErr.Code,
			UserMessage: appErr.UserMessage,
			Suggestion:  appErr.Suggestion,
			Detail:      appErr.Detail,
		}
	}
	return ConversionErrorMessage{
		TaskID:      taskID,
		Code:        "ERR_UNKNOWN",
		UserMessage: "转换任务启动失败。",
		Suggestion:  "请检查文件和输出目录后重试。",
		Detail:      err.Error(),
	}
}
