//go:build release && !runtimebundle

package runtimebundle

// A formal release build must include the embedded FFmpeg payload.
var _ = releaseBuildRequiresRuntimebundle
