package handler

import "testing"

func TestNormalizeConcurrencyAllowsSingleWorker(t *testing.T) {
	got := normalizeConcurrency(1, 1)
	if got != 1 {
		t.Fatalf("expected concurrency to keep 1, got %d", got)
	}
}
