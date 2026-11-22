package handler

import "testing"

func TestResolveEmbedMetadata(t *testing.T) {
	if !resolveEmbedMetadata(nil) {
		t.Fatal("nil should default to enabled")
	}
	enabled := true
	if !resolveEmbedMetadata(&enabled) {
		t.Fatal("explicit true should stay enabled")
	}
	disabled := false
	if resolveEmbedMetadata(&disabled) {
		t.Fatal("explicit false should disable")
	}
}
