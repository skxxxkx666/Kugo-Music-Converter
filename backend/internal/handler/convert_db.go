package handler

import (
	"fmt"
	"strings"

	"kugo-music-converter/internal/service"
)

func (h *ConvertHandler) getDBStatus() service.DBStatus {
	h.dbMu.RLock()
	if h.dbPath != "" && len(h.dbKeyMap) > 0 {
		defer h.dbMu.RUnlock()
		return service.DBStatus{Found: true, Path: h.dbPath, Source: h.dbSource}
	}
	h.dbMu.RUnlock()
	return service.DetectKGMusicDB(h.baseDir)
}

func (h *ConvertHandler) loadDBByPath(dbPath, source string) error {
	validation := service.ValidateDBPath(dbPath)
	if !validation.Valid {
		return fmt.Errorf("db path invalid: %s", validation.Reason)
	}
	keys, err := service.LoadDBKeyMap(validation.Path)
	if err != nil {
		return err
	}
	h.dbMu.Lock()
	h.dbPath = validation.Path
	h.dbSource = source
	h.dbKeyMap = keys
	h.dbMu.Unlock()
	return nil
}

// A-203: Return read-only shared key map reference instead of cloning per request.
func (h *ConvertHandler) getDBForRequest(requestPath string) (string, string, map[string]string, error) {
	if strings.TrimSpace(requestPath) != "" {
		validation := service.ValidateDBPath(requestPath)
		if !validation.Valid {
			return "", "", nil, NewAppError(ErrDBNotFound, "数据库路径无效", nil)
		}

		h.dbMu.RLock()
		alreadyLoaded := h.dbPath == validation.Path && len(h.dbKeyMap) > 0
		h.dbMu.RUnlock()

		if !alreadyLoaded {
			keys, err := service.LoadDBKeyMap(validation.Path)
			if err != nil {
				return "", "", nil, NewAppError(ErrDBNotFound, err.Error(), nil)
			}
			h.dbMu.Lock()
			h.dbPath = validation.Path
			h.dbSource = "manual"
			h.dbKeyMap = keys
			h.dbMu.Unlock()
			return validation.Path, "manual", keys, nil
		}
	}

	h.dbMu.RLock()
	if h.dbPath != "" && len(h.dbKeyMap) > 0 {
		path := h.dbPath
		source := h.dbSource
		keys := h.dbKeyMap
		h.dbMu.RUnlock()
		return path, source, keys, nil
	}
	h.dbMu.RUnlock()

	status := service.DetectKGMusicDB(h.baseDir)
	if !status.Found {
		return "", "", nil, NewAppError(ErrDBNotFound, "未检测到 KGMusicV3.db", nil)
	}
	keys, err := service.LoadDBKeyMap(status.Path)
	if err != nil {
		return "", "", nil, NewAppError(ErrDBNotFound, err.Error(), nil)
	}
	h.dbMu.Lock()
	h.dbPath = status.Path
	h.dbSource = status.Source
	h.dbKeyMap = keys
	h.dbMu.Unlock()
	return status.Path, status.Source, keys, nil
}
