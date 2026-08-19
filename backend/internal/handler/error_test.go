package handler

import (
	"strings"
	"testing"
)

func TestErrorCatalogDoesNotExposeLegacyLauncherWording(t *testing.T) {
	for code, meta := range errorCatalog {
		text := meta.userMessage + " " + meta.suggestion
		for _, legacy := range []string{"上传", "localhost", "start.hta", "start.bat", "本地服务", "用户目录"} {
			if strings.Contains(text, legacy) {
				t.Errorf("%s contains legacy wording %q: %s", code, legacy, text)
			}
		}
	}
}
