package service

import "errors"

var (
	ErrUnsupportedInput = errors.New("unsupported input format")
	ErrMissingKGGKey    = errors.New("kgg key missing")
	ErrDecryptProcess   = errors.New("decrypt process failed")
	ErrUnknownAudio     = errors.New("unknown audio format")
	ErrTranscodeProcess = errors.New("transcode process failed")
)
