package handler

import (
	"net/http"
)

type limitsResp struct {
	MaxFileCount  int `json:"maxFileCount"`
	MaxFileSizeMB int `json:"maxFileSizeMB"`
}

type configResp struct {
	DefaultOutputDir string     `json:"defaultOutputDir"`
	MissingTools     []string   `json:"missingTools"`
	DB               any        `json:"db"`
	Limits           limitsResp `json:"limits"`
	RuntimeReady     bool       `json:"runtimeReady"`
	SupportedFormats []string   `json:"supportedFormats"`
	SupportedExts    []string   `json:"supportedExts"`
}

func (h *ConvertHandler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	missingTools := h.runtimeMissingTools()
	db := h.getDBStatus()
	writeJSON(w, http.StatusOK, configResp{
		DefaultOutputDir: h.defaultOutputDir,
		MissingTools:     missingTools,
		DB:               db,
		Limits: limitsResp{
			MaxFileCount:  h.cfg.MaxFiles,
			MaxFileSizeMB: int(h.cfg.MaxFileSize / (1024 * 1024)),
		},
		RuntimeReady:     len(missingTools) == 0,
		SupportedFormats: supportedInputExts,
		SupportedExts:    supportedInputExts,
	})
}
