package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"kugo-music-converter/internal/service"
)

const maxUploadDBSize int64 = 100 << 20 // 100 MiB

type validateDBRequest struct {
	DBPath string `json:"dbPath"`
}

type validateDBResponse struct {
	Valid  bool   `json:"valid"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (h *ConvertHandler) HandleValidateDBPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
		return
	}

	var req validateDBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "请求体格式错误", err))
		return
	}

	validation := service.ValidateDBPath(req.DBPath)
	if !validation.Valid {
		writeJSON(w, http.StatusOK, validateDBResponse{Valid: false, Path: validation.Path, Reason: validation.Reason})
		return
	}

	if err := h.loadDBByPath(validation.Path, "manual"); err != nil {
		writeJSON(w, http.StatusOK, validateDBResponse{Valid: false, Path: validation.Path, Reason: "load_failed"})
		return
	}

	writeJSON(w, http.StatusOK, validateDBResponse{Valid: true, Path: validation.Path, Reason: "ok"})
}

func (h *ConvertHandler) HandleRedetectDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
		return
	}

	status := service.DetectKGMusicDB(h.baseDir)
	if status.Found {
		if err := h.loadDBByPath(status.Path, status.Source); err != nil {
			status.Found = false
			status.Path = ""
			status.Source = "missing"
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"db": status})
}

func (h *ConvertHandler) HandleUploadDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadDBSize)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "上传体积超限（最大 100MB）", err))
			return
		}
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "上传表单解析失败", err))
		return
	}

	files := r.MultipartForm.File["db"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "字段 db 缺失", nil))
		return
	}
	if files[0].Size > maxUploadDBSize {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "上传体积超限（最大 100MB）", nil))
		return
	}

	f, err := files[0].Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "打开数据库文件失败", err))
		return
	}
	defer f.Close()

	tmp, err := createTempFile("kgg-db-", ".db")
	if err != nil {
		writeError(w, http.StatusInternalServerError, NewAppError(ErrDBPathInvalid, "创建临时文件失败", err))
		return
	}
	defer removeQuiet(tmp)

	if _, err := copyStreamToFile(f, tmp); err != nil {
		writeError(w, http.StatusInternalServerError, NewAppError(ErrDBPathInvalid, "保存数据库文件失败", err))
		return
	}

	keys, err := service.LoadDBKeyMap(tmp)
	if err != nil {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, "数据库加载失败", err))
		return
	}

	h.dbMu.Lock()
	h.dbPath = "[uploaded]"
	h.dbSource = "manual"
	h.dbKeyMap = keys
	h.dbMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
