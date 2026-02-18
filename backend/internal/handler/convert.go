package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/logger"
	"kugo-music-converter/internal/service"
)

var (
	supportedInputExts = []string{".kgg", ".kgm", ".kgma", ".vpr", ".ncm"}
)

const (
	maxConcurrency        = 6
	serverShutdownTimeout = 15 * time.Second
)

type ConvertHandler struct {
	cfg            *config.Config
	decryptService *service.DecryptService

	baseDir          string
	publicDir        string
	ffmpegPath       string
	defaultOutputDir string

	dbMu     sync.RWMutex
	dbPath   string
	dbSource string
	dbKeyMap map[string]string

	shutdownCtx context.Context
}

func NewConvertHandler(cfg *config.Config) *ConvertHandler {
	baseDir := mustResolveBaseDir()
	publicDir := resolveDirectory(baseDir, cfg.PublicDir)
	ffmpegPath := resolveFile(baseDir, cfg.FFmpegBin)
	defaultOutputDir := resolveOutputDir(baseDir, cfg.DefaultOutput)

	h := &ConvertHandler{
		cfg:              cfg,
		decryptService:   service.NewDecryptService(cfg),
		baseDir:          baseDir,
		publicDir:        publicDir,
		ffmpegPath:       ffmpegPath,
		defaultOutputDir: defaultOutputDir,
		dbSource:         "missing",
		dbKeyMap:         map[string]string{},
		shutdownCtx:      context.Background(),
	}

	if st := service.DetectKGMusicDB(baseDir); st.Found {
		if err := h.loadDBByPath(st.Path, st.Source); err != nil {
			logger.Warnf("鑷姩鍔犺浇 KGMusicV3.db 澶辫触: %v", err)
		}
	}

	_ = os.MkdirAll(defaultOutputDir, 0o755)
	return h
}

func StartServer(ctx context.Context, cfg *config.Config) error {
	if ctx == nil {
		ctx = context.Background()
	}

	h := NewConvertHandler(cfg)
	h.setShutdownContext(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", h.HandleConfig)
	mux.HandleFunc("/api/convert", h.HandleConvert)
	mux.HandleFunc("/api/convert-stream", h.HandleConvertStream)
	mux.HandleFunc("/api/upload-db", h.HandleUploadDB)
	mux.HandleFunc("/api/pick-directory", h.HandlePickDirectory)
	mux.HandleFunc("/api/pick-db-file", h.HandlePickDBFile)
	mux.HandleFunc("/api/validate-db-path", h.HandleValidateDBPath)
	mux.HandleFunc("/api/redetect-db", h.HandleRedetectDB)
	mux.HandleFunc("/api/scan-folders", h.HandleScanFolders)
	mux.HandleFunc("/api/open-folder", h.HandleOpenFolder)

	fileServer := http.FileServer(http.Dir(h.publicDir))
	mux.Handle("/", fileServer)

	logger.Infof("鍚姩鏈嶅姟: addr=%s", cfg.Addr)
	logger.Infof("闈欐€佺洰褰? %s", h.publicDir)
	logger.Infof("FFmpeg 璺緞: %s", h.ffmpegPath)
	logger.Infof("榛樿杈撳嚭鐩綍: %s", h.defaultOutputDir)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           logRequest(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	stopShutdown := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Errorf("graceful shutdown failed: %v", err)
			}
		case <-stopShutdown:
		}
	}()

	err := srv.ListenAndServe()
	close(stopShutdown)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func mustResolveBaseDir() string {
	exe, err := os.Executable()
	if err == nil {
		resolved, err := filepath.EvalSymlinks(exe)
		if err == nil {
			return filepath.Dir(resolved)
		}
		return filepath.Dir(exe)
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "."
	}
	return cwd
}

func resolveDirectory(baseDir, raw string) string {
	candidates := []string{}
	if raw != "" {
		if filepath.IsAbs(raw) {
			candidates = append(candidates, raw)
		} else {
			candidates = append(candidates,
				filepath.Join(baseDir, raw),
				filepath.Join(baseDir, "..", raw),
				filepath.Join(baseDir, "..", "..", raw),
			)
			if cwd, err := os.Getwd(); err == nil {
				candidates = append(candidates, filepath.Join(cwd, raw))
			}
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "public"))
	}
	candidates = append(candidates,
		filepath.Join(baseDir, "public"),
		filepath.Join(baseDir, "..", "public"),
		filepath.Join(baseDir, "..", "..", "public"),
	)

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	if len(candidates) > 0 {
		abs, _ := filepath.Abs(candidates[0])
		return abs
	}
	return filepath.Join(baseDir, "public")
}

func resolveFile(baseDir, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "tools/ffmpeg.exe"
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}

	candidates := []string{
		filepath.Join(baseDir, trimmed),
		filepath.Join(baseDir, "..", trimmed),
		filepath.Join(baseDir, "..", "..", trimmed),
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, trimmed))
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	abs, _ := filepath.Abs(candidates[0])
	return abs
}

func resolveOutputDir(baseDir, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if filepath.IsAbs(trimmed) {
		return trimmed
	}

	if trimmed == "" || trimmed == "downloads" {
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			dl := filepath.Join(userProfile, "Downloads")
			if st, err := os.Stat(dl); err == nil && st.IsDir() {
				return dl
			}
		}
		if home := os.Getenv("HOME"); home != "" {
			dl := filepath.Join(home, "Downloads")
			if st, err := os.Stat(dl); err == nil && st.IsDir() {
				return dl
			}
		}
		trimmed = "output"
	}

	projectRootAbs, _ := filepath.Abs(filepath.Join(baseDir, "..", ".."))
	candidates := []string{
		filepath.Join(projectRootAbs, trimmed),
		filepath.Join(baseDir, trimmed),
		filepath.Join(baseDir, "..", trimmed),
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, trimmed))
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	abs, _ := filepath.Abs(candidates[0])
	return abs
}

func containsInputExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, item := range supportedInputExts {
		if ext == item {
			return true
		}
	}
	return false
}

func normalizeConcurrency(raw int, fallback int) int {
	if raw <= 0 {
		raw = fallback
	}
	if raw <= 0 {
		raw = 1
	}
	if raw > maxConcurrency {
		raw = maxConcurrency
	}
	return raw
}

func (h *ConvertHandler) runtimeMissingTools() []string {
	missing := make([]string, 0, 1)
	if st, err := os.Stat(h.ffmpegPath); err != nil || st.IsDir() {
		missing = append(missing, "tools/ffmpeg.exe")
	}
	return missing
}

func cloneKeyMap(src map[string]string) map[string]string {
	dup := make(map[string]string, len(src))
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func (h *ConvertHandler) setShutdownContext(ctx context.Context) {
	if ctx == nil {
		h.shutdownCtx = context.Background()
		return
	}
	h.shutdownCtx = ctx
}

func (h *ConvertHandler) isShuttingDown() bool {
	if h.shutdownCtx == nil {
		return false
	}
	select {
	case <-h.shutdownCtx.Done():
		return true
	default:
		return false
	}
}

func (h *ConvertHandler) contextWithShutdown(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	combined, cancel := context.WithCancel(ctx)
	if h.shutdownCtx == nil {
		return combined, cancel
	}
	go func() {
		select {
		case <-h.shutdownCtx.Done():
			cancel()
		case <-combined.Done():
		}
	}()
	return combined, cancel
}

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
			if err := h.loadDBByPath(validation.Path, "manual"); err != nil {
				return "", "", nil, NewAppError(ErrDBNotFound, err.Error(), nil)
			}
		}
	}

	h.dbMu.RLock()
	if h.dbPath != "" && len(h.dbKeyMap) > 0 {
		path := h.dbPath
		source := h.dbSource
		keys := cloneKeyMap(h.dbKeyMap)
		h.dbMu.RUnlock()
		return path, source, keys, nil
	}
	h.dbMu.RUnlock()

	status := service.DetectKGMusicDB(h.baseDir)
	if !status.Found {
		return "", "", nil, NewAppError(ErrDBNotFound, "鏈娴嬪埌 KGMusicV3.db", nil)
	}
	if err := h.loadDBByPath(status.Path, status.Source); err != nil {
		return "", "", nil, NewAppError(ErrDBNotFound, err.Error(), nil)
	}

	h.dbMu.RLock()
	defer h.dbMu.RUnlock()
	return h.dbPath, h.dbSource, cloneKeyMap(h.dbKeyMap), nil
}

func detectErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ErrCancelled
	case errors.Is(err, service.ErrUnsupportedInput):
		return ErrUnsupportedFormat
	case errors.Is(err, service.ErrTranscodeProcess):
		return ErrTranscodeFailed
	case errors.Is(err, service.ErrMissingKGGKey):
		return ErrDecryptKeyExpired
	case errors.Is(err, service.ErrUnknownAudio), errors.Is(err, service.ErrDecryptProcess):
		return ErrDecryptFailed
	default:
		return ErrDecryptFailed
	}
}

func toBatchFileError(err error) *service.BatchFileError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return &service.BatchFileError{
			Code:        appErr.Code,
			UserMessage: appErr.UserMessage,
			Suggestion:  appErr.Suggestion,
			Severity:    appErr.Severity,
			Detail:      appErr.Detail,
		}
	}

	mapped := NewAppError(detectErrorCode(err), err.Error(), nil)
	return &service.BatchFileError{
		Code:        mapped.Code,
		UserMessage: mapped.UserMessage,
		Suggestion:  mapped.Suggestion,
		Severity:    mapped.Severity,
		Detail:      mapped.Detail,
	}
}
