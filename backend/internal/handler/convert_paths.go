package handler

import (
	"os"
	"path/filepath"
	"strings"

	"kugo-music-converter/internal/logger"
)

func logPathCandidates(kind string, candidates []string) {
	for i, candidate := range candidates {
		logger.Debugf("%s candidate[%d]: %s", kind, i, candidate)
	}
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
	logPathCandidates("resolveDirectory", candidates)

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			abs, _ := filepath.Abs(c)
			logger.Debugf("resolveDirectory selected: %s", abs)
			return abs
		}
	}
	if len(candidates) > 0 {
		abs, _ := filepath.Abs(candidates[0])
		logger.Debugf("resolveDirectory fallback: %s", abs)
		return abs
	}
	fallback := filepath.Join(baseDir, "public")
	logger.Debugf("resolveDirectory default fallback: %s", fallback)
	return fallback
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
	logPathCandidates("resolveFile", candidates)

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			logger.Debugf("resolveFile selected: %s", abs)
			return abs
		}
	}

	abs, _ := filepath.Abs(candidates[0])
	logger.Debugf("resolveFile fallback: %s", abs)
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
