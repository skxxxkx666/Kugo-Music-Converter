package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"kugo-music-converter/internal/service"
)

func validateLocalFolderPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("目录路径不能为空")
	}
	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, `//`) {
		return "", fmt.Errorf("不支持网络共享路径")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("路径无效")
	}

	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(absPath)
		if len(volume) != 2 || volume[1] != ':' {
			return "", fmt.Errorf("仅允许本地磁盘路径")
		}
		for i, r := range absPath {
			if strings.ContainsRune(`<>:"|?*`, r) {
				if r == ':' && i == 1 {
					continue
				}
				return "", fmt.Errorf("路径包含非法字符")
			}
		}
	}

	return absPath, nil
}

func runPowershell(command string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", command)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", NewAppError(ErrFolderPicker, msg, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (h *ConvertHandler) HandlePickDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
		return
	}
	if runtime.GOOS != "windows" {
		writeError(w, http.StatusBadRequest, NewAppError(ErrFolderPicker, "仅支持 Windows 目录选择器", nil))
		return
	}

	path, err := runPowershell(
		`Add-Type -AssemblyName System.Windows.Forms; ` +
			`$d = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
			`$d.Description = '选择输出目录'; ` +
			`$d.ShowNewFolderButton = $true; ` +
			`if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $d.SelectedPath }`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

func (h *ConvertHandler) HandlePickDBFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
		return
	}
	if runtime.GOOS != "windows" {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPicker, "仅支持 Windows 文件选择器", nil))
		return
	}

	path, err := runPowershell(
		`Add-Type -AssemblyName System.Windows.Forms; ` +
			`$d = New-Object System.Windows.Forms.OpenFileDialog; ` +
			`$d.Title = '选择 KGMusicV3.db'; ` +
			`$d.Filter = 'KGMusicV3.db|KGMusicV3.db|SQLite 文件 (*.db)|*.db|所有文件 (*.*)|*.*'; ` +
			`$d.Multiselect = $false; ` +
			`$d.CheckFileExists = $true; ` +
			`if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $d.FileName }`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, NewAppError(ErrDBPicker, err.Error(), err))
		return
	}

	if strings.TrimSpace(path) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"path": "", "valid": false, "reason": "cancelled"})
		return
	}

	validation := service.ValidateDBPath(path)
	if !validation.Valid {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, validation.Reason, nil))
		return
	}

	if err := h.loadDBByPath(validation.Path, "manual"); err != nil {
		writeError(w, http.StatusBadRequest, NewAppError(ErrDBPathInvalid, err.Error(), err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":   validation.Path,
		"valid":  true,
		"reason": "ok",
		"db":     h.getDBStatus(),
	})
}

func (h *ConvertHandler) HandleOpenFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, NewAppError("ERR_UNKNOWN", "method not allowed", nil))
		return
	}
	if runtime.GOOS != "windows" {
		writeError(w, http.StatusBadRequest, NewAppError(ErrFolderPicker, "仅支持 Windows", nil))
		return
	}

	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, NewAppError("ERR_UNKNOWN", "请求格式错误", err))
		return
	}

	dirPath := strings.TrimSpace(body.Path)
	if dirPath == "" {
		dirPath = h.defaultOutputDir
	}

	absPath, err := validateLocalFolderPath(dirPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, NewAppError(ErrFolderPicker, "路径无效", err))
		return
	}

	if err := os.MkdirAll(absPath, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, NewAppError(ErrFolderPicker, "无法创建目录", err))
		return
	}

	cmd := exec.Command("explorer.exe", absPath)
	_ = cmd.Start()

	writeJSON(w, http.StatusOK, map[string]any{"path": absPath, "opened": true})
}
