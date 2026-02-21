package handler

import (
	"net/http"
	"runtime"
	"time"
)

const serverVersion = "v0.2.3"

type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	GoVersion string `json:"goVersion"`
}

func (h *ConvertHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	uptime := "0s"
	if !h.startedAt.IsZero() {
		uptime = time.Since(h.startedAt).Truncate(time.Second).String()
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Version:   serverVersion,
		Uptime:    uptime,
		GoVersion: runtime.Version(),
	})
}
