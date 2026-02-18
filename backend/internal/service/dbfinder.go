package service

import (
	"os"
	"path/filepath"
	"strings"

	"kugo-music-converter/internal/algo/kgg"
)

type DBStatus struct {
	Found  bool   `json:"found"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

type DBValidation struct {
	Valid  bool   `json:"valid"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func ValidateDBPath(candidate string) DBValidation {
	raw := strings.TrimSpace(candidate)
	if raw == "" {
		return DBValidation{Valid: false, Reason: "empty"}
	}

	abs, err := filepath.Abs(raw)
	if err != nil {
		return DBValidation{Valid: false, Reason: "invalid_path"}
	}

	st, err := os.Stat(abs)
	if err != nil {
		return DBValidation{Valid: false, Path: abs, Reason: "not_found"}
	}
	if !st.Mode().IsRegular() {
		return DBValidation{Valid: false, Path: abs, Reason: "not_file"}
	}
	if strings.ToLower(filepath.Base(abs)) != "kgmusicv3.db" {
		return DBValidation{Valid: false, Path: abs, Reason: "invalid_name"}
	}

	return DBValidation{Valid: true, Path: abs, Reason: "ok"}
}

func DetectKGMusicDB(baseDir string) DBStatus {
	candidates := []struct {
		path   string
		source string
	}{
		{path: filepath.Join(baseDir, "KGMusicV3.db"), source: "project"},
		{path: filepath.Join(baseDir, "..", "KGMusicV3.db"), source: "project"},
		{path: filepath.Join(baseDir, "..", "..", "KGMusicV3.db"), source: "project"},
	}

	if appData := os.Getenv("APPDATA"); appData != "" {
		candidates = append(candidates,
			struct {
				path   string
				source string
			}{path: filepath.Join(appData, "KuGou", "KGMusicV3.db"), source: "appdata"},
			struct {
				path   string
				source string
			}{path: filepath.Join(appData, "KuGou8", "KGMusicV3.db"), source: "appdata"},
		)
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates,
			struct {
				path   string
				source string
			}{path: filepath.Join(localAppData, "KuGou", "KGMusicV3.db"), source: "localappdata"},
			struct {
				path   string
				source string
			}{path: filepath.Join(localAppData, "KuGou8", "KGMusicV3.db"), source: "localappdata"},
		)
	}

	for _, item := range candidates {
		v := ValidateDBPath(item.path)
		if v.Valid {
			return DBStatus{Found: true, Path: v.Path, Source: item.source}
		}
	}

	return DBStatus{Found: false, Source: "missing"}
}

func LoadDBKeyMap(dbPath string) (map[string]string, error) {
	decPath, cleanup, err := kgg.DecryptKGDatabaseToFile(dbPath)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return kgg.ReadShareFileItems(decPath)
}
