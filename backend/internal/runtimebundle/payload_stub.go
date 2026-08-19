//go:build !runtimebundle

package runtimebundle

func FFmpegPayload() ([]byte, string) {
	return nil, ""
}
