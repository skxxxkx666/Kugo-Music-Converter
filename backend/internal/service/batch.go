package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type BatchItem struct {
	Path       string
	OriginPath string
	Name       string
	Size       int64
	Temporary  bool
	Current    int
}

type BatchFileError struct {
	Code        string `json:"code"`
	UserMessage string `json:"userMessage"`
	Suggestion  string `json:"suggestion,omitempty"`
	Severity    string `json:"severity"`
	Detail      string `json:"detail,omitempty"`
}

type BatchProgressEvent struct {
	Phase   string `json:"phase"`
	File    string `json:"file"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Percent int    `json:"percent"`
}

type BatchFileDoneEvent struct {
	File    string          `json:"file"`
	Input   string          `json:"input,omitempty"`
	Status  string          `json:"status"`
	Output  string          `json:"output,omitempty"`
	Error   *BatchFileError `json:"error,omitempty"`
	Current int             `json:"current"`
	Total   int             `json:"total"`
	Percent int             `json:"percent"`
}

type BatchSummary struct {
	Success      int                  `json:"success"`
	Failed       int                  `json:"failed"`
	Total        int                  `json:"total"`
	OutputDir    string               `json:"outputDir"`
	DurationMs   int64                `json:"durationMs"`
	Cancelled    bool                 `json:"cancelled"`
	OutputFormat string               `json:"outputFormat"`
	MP3Quality   int                  `json:"mp3Quality"`
	Results      []BatchFileDoneEvent `json:"results"`
}

type BatchOptions struct {
	Items        []BatchItem
	Concurrency  int
	OutputDir    string
	OutputFormat string
	MP3Quality   int
	ShouldStop   func() bool
	Convert      func(context.Context, BatchItem, func(phase string, filePercent int)) (string, error)
	ErrorMapper  func(error) *BatchFileError
	OnProgress   func(BatchProgressEvent)
	OnFileDone   func(BatchFileDoneEvent)
}

func computePercent(doneFiles int, filePercent int, total int) int {
	if total <= 0 {
		return 0
	}
	if filePercent < 0 {
		filePercent = 0
	}
	if filePercent > 100 {
		filePercent = 100
	}
	return ((doneFiles * 100) + filePercent) * 100 / (total * 100)
}

func RunBatch(ctx context.Context, opts BatchOptions) BatchSummary {
	started := time.Now()
	total := len(opts.Items)
	if total == 0 {
		return BatchSummary{Total: 0, OutputDir: opts.OutputDir, OutputFormat: opts.OutputFormat, MP3Quality: opts.MP3Quality}
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > total {
		concurrency = total
	}

	var completed int32
	var success int32
	var failed int32
	var cancelled atomic.Bool

	results := make([]BatchFileDoneEvent, total)
	jobs := make(chan BatchItem, total)
	for _, item := range opts.Items {
		jobs <- item
	}
	close(jobs)

	var mu sync.Mutex
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for item := range jobs {
			if opts.ShouldStop != nil && opts.ShouldStop() {
				cancelled.Store(true)
				continue
			}
			if ctx.Err() != nil {
				cancelled.Store(true)
				continue
			}

			progress := func(phase string, filePercent int) {
				if opts.OnProgress == nil {
					return
				}
				done := int(atomic.LoadInt32(&completed))
				opts.OnProgress(BatchProgressEvent{
					Phase:   phase,
					File:    item.Name,
					Current: item.Current,
					Total:   total,
					Percent: computePercent(done, filePercent, total),
				})
			}

			outputPath, err := opts.Convert(ctx, item, progress)
			doneNow := int(atomic.AddInt32(&completed, 1))

			evt := BatchFileDoneEvent{
				File:    item.Name,
				Input:   item.OriginPath,
				Current: item.Current,
				Total:   total,
				Percent: computePercent(doneNow, 0, total),
			}

			if err != nil {
				atomic.AddInt32(&failed, 1)
				evt.Status = "error"
				if opts.ErrorMapper != nil {
					evt.Error = opts.ErrorMapper(err)
				}
				if evt.Error != nil && evt.Error.Code == "ERR_CANCELLED" {
					cancelled.Store(true)
				}
			} else {
				atomic.AddInt32(&success, 1)
				evt.Status = "ok"
				evt.Output = outputPath
			}

			mu.Lock()
			results[item.Current-1] = evt
			mu.Unlock()

			if opts.OnFileDone != nil {
				opts.OnFileDone(evt)
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()

	for i, evt := range results {
		if evt.File != "" {
			continue
		}
		item := opts.Items[i]
		evt = BatchFileDoneEvent{
			File:    item.Name,
			Input:   item.OriginPath,
			Status:  "error",
			Current: item.Current,
			Total:   total,
			Percent: computePercent(int(atomic.LoadInt32(&completed)), 0, total),
			Error: &BatchFileError{
				Code:        "ERR_CANCELLED",
				UserMessage: "转换已取消",
				Suggestion:  "可重新发起转换任务",
				Severity:    "warning",
			},
		}
		results[i] = evt
		atomic.AddInt32(&failed, 1)
	}

	return BatchSummary{
		Success:      int(atomic.LoadInt32(&success)),
		Failed:       int(atomic.LoadInt32(&failed)),
		Total:        total,
		OutputDir:    opts.OutputDir,
		DurationMs:   time.Since(started).Milliseconds(),
		Cancelled:    cancelled.Load() || (opts.ShouldStop != nil && opts.ShouldStop()) || ctx.Err() != nil,
		OutputFormat: opts.OutputFormat,
		MP3Quality:   opts.MP3Quality,
		Results:      results,
	}
}
