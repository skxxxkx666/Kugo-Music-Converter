package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"kugo-music-converter/internal/config"
	"kugo-music-converter/internal/logger"
	"kugo-music-converter/internal/qmckey"
	"kugo-music-converter/internal/service"
)

var (
	supportedInputExts = []string{
		".kgg", ".kgm", ".kgma", ".vpr", ".ncm", ".kwm",
		".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg", ".tkm",
	}
	modernQMCInputExts = []string{".mflac", ".mgg"}
)

const (
	maxConcurrencyHardCap = 12
	minConcurrency        = 1
	serverShutdownTimeout = 15 * time.Second
	nonSSEWriteTimeout    = 30 * time.Second
)

var appVersionAttrPattern = regexp.MustCompile(`data-app-version="[^"]*"`)

type ConvertHandler struct {
	cfg               *config.Config
	decryptService    *service.DecryptService
	qmcKeyResolver    qmckey.BatchResolver
	supportsModernQMC bool
	startedAt         time.Time
	version           string

	baseDir          string
	publicDir        string
	ffmpegPath       string
	defaultOutputDir string
	indexHTML        []byte // cached index.html with version injected

	dbMu     sync.RWMutex
	dbPath   string
	dbSource string
	dbKeyMap map[string]string

	previewMu    sync.RWMutex
	previewFiles map[string]time.Time

	ffmpegProbeMu   sync.Mutex
	ffmpegReady     bool
	ffmpegMessage   string
	ffmpegCheckedAt time.Time

	shutdownCtx context.Context
}

func NewConvertHandler(cfg *config.Config, appVersion string) *ConvertHandler {
	baseDir := mustResolveBaseDir()
	publicDir := resolveDirectory(baseDir, cfg.PublicDir)
	ffmpegPath := resolveFile(baseDir, cfg.FFmpegBin)
	defaultOutputDir := resolveOutputDir(baseDir, cfg.DefaultOutput)
	version := strings.TrimSpace(appVersion)
	if version == "" {
		version = "dev"
	}

	h := &ConvertHandler{
		cfg:              cfg,
		decryptService:   service.NewDecryptService(cfg),
		startedAt:        time.Now(),
		version:          version,
		baseDir:          baseDir,
		publicDir:        publicDir,
		ffmpegPath:       ffmpegPath,
		defaultOutputDir: defaultOutputDir,
		dbSource:         "missing",
		dbKeyMap:         map[string]string{},
		previewFiles:     map[string]time.Time{},
		shutdownCtx:      context.Background(),
	}

	if st := service.DetectKGMusicDB(baseDir); st.Found {
		if err := h.loadDBByPath(st.Path, st.Source); err != nil {
			logger.Warnf("自动加载 KGMusicV3.db 失败: %v", err)
		}
	}

	_ = os.MkdirAll(defaultOutputDir, 0o755)

	// A-602: cache index.html with version injected.
	if raw, err := os.ReadFile(filepath.Join(publicDir, "index.html")); err == nil {
		injected := appVersionAttrPattern.ReplaceAllString(
			string(raw),
			fmt.Sprintf(`data-app-version="%s"`, version),
		)
		h.indexHTML = []byte(injected)
	}

	return h
}

func NewDesktopConvertHandler(cfg *config.Config, appVersion string) *ConvertHandler {
	h := NewConvertHandler(cfg, appVersion)
	h.qmcKeyResolver = qmckey.NewDefaultResolver()
	h.supportsModernQMC = true
	return h
}

func StartServer(ctx context.Context, cfg *config.Config, appVersion string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	h := NewConvertHandler(cfg, appVersion)
	h.setShutdownContext(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", h.HandleConfig)
	mux.HandleFunc("/api/health", h.HandleHealth)
	mux.HandleFunc("/api/convert", h.HandleConvert)
	mux.HandleFunc("/api/convert-stream", h.HandleConvertStream)
	mux.HandleFunc("/api/upload-db", h.HandleUploadDB)
	mux.HandleFunc("/api/pick-directory", h.HandlePickDirectory)
	mux.HandleFunc("/api/pick-db-file", h.HandlePickDBFile)
	mux.HandleFunc("/api/validate-db-path", h.HandleValidateDBPath)
	mux.HandleFunc("/api/redetect-db", h.HandleRedetectDB)
	mux.HandleFunc("/api/scan-folders", h.HandleScanFolders)
	mux.HandleFunc("/api/open-folder", h.HandleOpenFolder)
	mux.HandleFunc("/api/preview-file", h.HandlePreviewFile)
	mux.HandleFunc("/api/check-update", h.HandleCheckUpdate)

	fileServer := http.FileServer(http.Dir(h.publicDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/" || r.URL.Path == "/index.html") && len(h.indexHTML) > 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(h.indexHTML)
			return
		}
		fileServer.ServeHTTP(w, r)
	}))

	logger.Infof("启动服务: addr=%s", cfg.Addr)
	logger.Infof("静态目录: %s", h.publicDir)
	logger.Infof("FFmpeg 路径: %s", h.ffmpegPath)
	logger.Infof("默认输出目录: %s", h.defaultOutputDir)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           withWriteTimeout(logRequest(withLocalOriginGuard(mux)), nonSSEWriteTimeout, isSSERequest),
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
