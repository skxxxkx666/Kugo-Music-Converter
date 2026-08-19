//go:build runtimebundle

package runtimebundle

import (
	_ "embed"
	"strings"
)

//go:embed payload/ffmpeg.exe.gz
var ffmpegPayload []byte

//go:embed payload/ffmpeg.sha256
var ffmpegSHA256 string

func FFmpegPayload() ([]byte, string) {
	return ffmpegPayload, strings.TrimSpace(ffmpegSHA256)
}
