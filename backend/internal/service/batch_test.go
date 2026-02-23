package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── helpers ──────────────────────────────────────────────────────────

func makeItems(n int) []BatchItem {
	items := make([]BatchItem, n)
	for i := range items {
		items[i] = BatchItem{
			Path:       fmt.Sprintf("/fake/path/%d.kgg", i),
			OriginPath: fmt.Sprintf("/fake/path/%d.kgg", i),
			Name:       fmt.Sprintf("file_%d.kgg", i),
			Size:       1024,
			Current:    i + 1,
		}
	}
	return items
}

func noopErrorMapper(err error) *BatchFileError {
	return &BatchFileError{
		Code:        "ERR_TEST",
		UserMessage: err.Error(),
		Severity:    "error",
	}
}

// ── Tests ────────────────────────────────────────────────────────────

func TestRunBatch_Empty(t *testing.T) {
	summary := RunBatch(context.Background(), BatchOptions{
		Items:       nil,
		Concurrency: 2,
	})
	if summary.Total != 0 {
		t.Fatalf("expected Total=0, got %d", summary.Total)
	}
	if summary.Success != 0 || summary.Failed != 0 {
		t.Fatalf("expected 0 success/failed for empty batch")
	}
}

func TestRunBatch_AllSuccess(t *testing.T) {
	items := makeItems(5)

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: 2,
		OutputDir:   "/out",
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			if progress != nil {
				progress("decrypt", 50)
				progress("transcode", 100)
			}
			return fmt.Sprintf("/out/%s", item.Name), nil
		},
		ErrorMapper: noopErrorMapper,
	})

	if summary.Total != 5 {
		t.Fatalf("expected Total=5, got %d", summary.Total)
	}
	if summary.Success != 5 {
		t.Fatalf("expected Success=5, got %d", summary.Success)
	}
	if summary.Failed != 0 {
		t.Fatalf("expected Failed=0, got %d", summary.Failed)
	}
	if summary.Cancelled {
		t.Fatal("expected Cancelled=false")
	}
	for i, r := range summary.Results {
		if r.Status != "ok" {
			t.Fatalf("result[%d] expected status=ok, got %s", i, r.Status)
		}
		if r.Output == "" {
			t.Fatalf("result[%d] expected non-empty output", i)
		}
	}
}

func TestRunBatch_AllFailed(t *testing.T) {
	items := makeItems(3)
	testErr := errors.New("decrypt failed")

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: 1,
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			return "", testErr
		},
		ErrorMapper: noopErrorMapper,
	})

	if summary.Success != 0 {
		t.Fatalf("expected Success=0, got %d", summary.Success)
	}
	if summary.Failed != 3 {
		t.Fatalf("expected Failed=3, got %d", summary.Failed)
	}
	for i, r := range summary.Results {
		if r.Status != "error" {
			t.Fatalf("result[%d] expected status=error, got %s", i, r.Status)
		}
		if r.Error == nil {
			t.Fatalf("result[%d] expected non-nil error", i)
		}
	}
}

func TestRunBatch_PartialFailure(t *testing.T) {
	items := makeItems(4)

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: 1,
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			if item.Current%2 == 0 {
				return "", errors.New("even items fail")
			}
			return "/out/" + item.Name, nil
		},
		ErrorMapper: noopErrorMapper,
	})

	if summary.Success != 2 {
		t.Fatalf("expected Success=2, got %d", summary.Success)
	}
	if summary.Failed != 2 {
		t.Fatalf("expected Failed=2, got %d", summary.Failed)
	}
}

func TestRunBatch_ConcurrencyRespected(t *testing.T) {
	items := makeItems(10)
	maxConcurrency := 3

	var running int32
	var maxRunning int32

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: maxConcurrency,
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			cur := atomic.AddInt32(&running, 1)
			defer atomic.AddInt32(&running, -1)

			// track peak concurrency
			for {
				old := atomic.LoadInt32(&maxRunning)
				if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			return "/out/" + item.Name, nil
		},
		ErrorMapper: noopErrorMapper,
	})

	if summary.Success != 10 {
		t.Fatalf("expected Success=10, got %d", summary.Success)
	}
	peak := atomic.LoadInt32(&maxRunning)
	if peak > int32(maxConcurrency) {
		t.Fatalf("peak concurrency %d exceeded limit %d", peak, maxConcurrency)
	}
	if peak < 2 {
		t.Logf("warning: peak concurrency was only %d, expected >=2 with 10 items", peak)
	}
}

func TestRunBatch_ContextCancel(t *testing.T) {
	items := makeItems(20)
	ctx, cancel := context.WithCancel(context.Background())

	var processed int32

	summary := RunBatch(ctx, BatchOptions{
		Items:       items,
		Concurrency: 1,
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			n := atomic.AddInt32(&processed, 1)
			if n >= 3 {
				cancel()
			}
			if ctx.Err() != nil {
				return "", fmt.Errorf("cancelled")
			}
			time.Sleep(5 * time.Millisecond)
			return "/out/" + item.Name, nil
		},
		ErrorMapper: noopErrorMapper,
	})

	if summary.Total != 20 {
		t.Fatalf("expected Total=20, got %d", summary.Total)
	}
	if summary.Success+summary.Failed != 20 {
		t.Fatalf("expected success+failed=20, got %d+%d=%d", summary.Success, summary.Failed, summary.Success+summary.Failed)
	}
	if !summary.Cancelled {
		t.Fatal("expected Cancelled=true after context cancel")
	}
}

func TestRunBatch_ShouldStop(t *testing.T) {
	items := makeItems(10)
	var processed int32

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: 1,
		ShouldStop: func() bool {
			return atomic.LoadInt32(&processed) >= 2
		},
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			atomic.AddInt32(&processed, 1)
			time.Sleep(5 * time.Millisecond)
			return "/out/" + item.Name, nil
		},
		ErrorMapper: noopErrorMapper,
	})

	if summary.Total != 10 {
		t.Fatalf("expected Total=10, got %d", summary.Total)
	}
	if !summary.Cancelled {
		t.Fatal("expected Cancelled=true after ShouldStop")
	}
	// At most a few items should have been processed before stop kicked in
	if summary.Success > 5 {
		t.Fatalf("expected at most 5 success (early stop), got %d", summary.Success)
	}
}

func TestRunBatch_ProgressCallbacks(t *testing.T) {
	items := makeItems(3)

	var progressMu sync.Mutex
	var progressEvents []BatchProgressEvent
	var fileDoneEvents []BatchFileDoneEvent

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: 1,
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			if progress != nil {
				progress("decrypt", 50)
				progress("transcode", 100)
			}
			return "/out/" + item.Name, nil
		},
		ErrorMapper: noopErrorMapper,
		OnProgress: func(event BatchProgressEvent) {
			progressMu.Lock()
			progressEvents = append(progressEvents, event)
			progressMu.Unlock()
		},
		OnFileDone: func(event BatchFileDoneEvent) {
			progressMu.Lock()
			fileDoneEvents = append(fileDoneEvents, event)
			progressMu.Unlock()
		},
	})

	if summary.Success != 3 {
		t.Fatalf("expected Success=3, got %d", summary.Success)
	}
	if len(progressEvents) == 0 {
		t.Fatal("expected at least one progress event")
	}
	if len(fileDoneEvents) != 3 {
		t.Fatalf("expected 3 file-done events, got %d", len(fileDoneEvents))
	}
	for _, evt := range fileDoneEvents {
		if evt.Status != "ok" {
			t.Fatalf("file-done event expected status=ok, got %s", evt.Status)
		}
	}
}

func TestRunBatch_DurationTracked(t *testing.T) {
	items := makeItems(2)

	summary := RunBatch(context.Background(), BatchOptions{
		Items:       items,
		Concurrency: 1,
		Convert: func(ctx context.Context, item BatchItem, progress func(string, int)) (string, error) {
			time.Sleep(20 * time.Millisecond)
			return "/out/" + item.Name, nil
		},
	})

	if summary.DurationMs < 20 {
		t.Fatalf("expected DurationMs >= 20, got %d", summary.DurationMs)
	}
}

func TestComputePercentByFiles(t *testing.T) {
	tests := []struct {
		done, filePct, total int
		want                 int
	}{
		{0, 0, 10, 0},
		{5, 0, 10, 50},
		{10, 0, 10, 100},
		{0, 50, 10, 5},
		{9, 100, 10, 100},
		{0, 0, 0, 0},     // zero total
		{0, -5, 10, 0},   // negative clamp
		{0, 200, 10, 10}, // clamp to 100
	}
	for _, tc := range tests {
		got := computePercentByFiles(tc.done, tc.filePct, tc.total)
		if got != tc.want {
			t.Errorf("computePercentByFiles(%d, %d, %d) = %d, want %d",
				tc.done, tc.filePct, tc.total, got, tc.want)
		}
	}
}

func TestComputePercentByBytes(t *testing.T) {
	percent, processed := computePercent(50, 100, 50, 200, 0, 2)
	if percent != 50 || processed != 100 {
		t.Fatalf("expected percent=50 processed=100, got percent=%d processed=%d", percent, processed)
	}

	percent, processed = computePercent(200, 100, 100, 200, 2, 2)
	if percent != 100 || processed != 200 {
		t.Fatalf("expected percent=100 processed=200, got percent=%d processed=%d", percent, processed)
	}
}
