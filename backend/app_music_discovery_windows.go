//go:build windows

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	maxMusicConfigFileSize = 8 << 20
	maxMusicConfigFiles    = 128
	maxMusicConfigDepth    = 4
)

var windowsAbsolutePathPattern = regexp.MustCompile(`(?i)[a-z]:\\[^<>:"|?*\x00-\x1f]{1,500}`)

func defaultMusicSearchSources() []musicSearchSource {
	musicRoots := windowsMusicRoots()

	kugouDirectories := make([]musicSearchDirectory, 0, 12)
	for _, configPath := range kugouConfigPaths() {
		downloadPath := readMusicINIValue(configPath, "DownloadConfigSection", "DownloadPath")
		if downloadPath == "" {
			continue
		}
		downloadPath = filepath.Clean(downloadPath)
		if strings.EqualFold(filepath.Base(downloadPath), "KugouMusic") {
			kugouDirectories = append(kugouDirectories, musicSearchDirectory{Path: downloadPath, Recursive: true})
			continue
		}
		// KuGou may save finished files at the configured root while encrypted
		// downloads are commonly kept in its KugouMusic child directory.
		kugouDirectories = append(kugouDirectories,
			musicSearchDirectory{Path: downloadPath},
			musicSearchDirectory{Path: filepath.Join(downloadPath, "KugouMusic"), Recursive: true},
		)
	}
	kugouDirectories = append(kugouDirectories, musicDirectoriesUnder(musicRoots,
		"KuGou", "KugouMusic", "KuGouMusic")...)

	neteaseDirectories := musicDirectoriesUnder(musicRoots, "CloudMusic", "NetEase", "网易云音乐")
	neteaseDirectories = append(neteaseDirectories, musicDirectoriesAtFixedDriveRoots(
		"CloudMusic", filepath.Join("CloudMusic", "VipSongsDownload"), "网易云音乐")...)
	neteaseDirectories = append(neteaseDirectories, musicDirectoriesFromConfigRoots(neteaseConfigRoots())...)

	kuwoDirectories := musicDirectoriesUnder(musicRoots, "KwDownload", "KuWo", "KuwoMusic", "酷我音乐")
	kuwoDirectories = append(kuwoDirectories, musicDirectoriesAtFixedDriveRoots(
		"KwDownload", "KuWo", "KuwoMusic", filepath.Join("KuwoMusic", "Download"), "酷我音乐")...)
	kuwoDirectories = append(kuwoDirectories, musicDirectoriesFromConfigRoots(kuwoConfigRoots())...)

	qqDirectories := musicDirectoriesUnder(musicRoots, "QQMusic", "QQ音乐", filepath.Join("Tencent", "QQMusic"))
	qqDirectories = append(qqDirectories, musicDirectoriesAtFixedDriveRoots(
		"QQMusic", filepath.Join("QQMusic", "Song"), filepath.Join("Tencent", "QQMusic"), "QQ音乐")...)
	qqDirectories = append(qqDirectories, musicDirectoriesFromConfigRoots(qqMusicConfigRoots())...)

	return []musicSearchSource{
		{
			ID:          "kugou",
			Name:        "酷狗音乐",
			Icon:        "i-brand-kugou",
			Directories: kugouDirectories,
		},
		{
			ID:          "netease",
			Name:        "网易云音乐",
			Icon:        "i-brand-netease",
			Directories: neteaseDirectories,
		},
		{
			ID:          "kuwo",
			Name:        "酷我音乐",
			IconImg:     "./assets/brand-kuwo.png",
			Directories: kuwoDirectories,
		},
		{
			ID:          "qq",
			Name:        "QQ 音乐",
			Icon:        "i-brand-qq",
			Directories: qqDirectories,
		},
	}
}

func neteaseConfigRoots() []string {
	return musicClientConfigRoots(
		[]string{"LOCALAPPDATA", "APPDATA"},
		filepath.Join("NetEase", "CloudMusic"),
		filepath.Join("Netease", "CloudMusic"),
	)
}

func kuwoConfigRoots() []string {
	return musicClientConfigRoots(
		[]string{"APPDATA", "LOCALAPPDATA"},
		"Kuwo", "KuwoMusic", "kwmusic",
	)
}

func qqMusicConfigRoots() []string {
	return musicClientConfigRoots(
		[]string{"APPDATA", "LOCALAPPDATA"},
		filepath.Join("Tencent", "QQMusic"), "QQMusic",
	)
}

func musicClientConfigRoots(environmentNames []string, relativePaths ...string) []string {
	roots := make([]string, 0, len(environmentNames)*len(relativePaths))
	seen := make(map[string]struct{})
	for _, environmentName := range environmentNames {
		base := strings.TrimSpace(os.Getenv(environmentName))
		if base == "" {
			continue
		}
		for _, relativePath := range relativePaths {
			root := filepath.Clean(filepath.Join(base, relativePath))
			key := strings.ToLower(root)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			roots = append(roots, root)
		}
	}
	return roots
}

func musicDirectoriesAtFixedDriveRoots(names ...string) []musicSearchDirectory {
	directories := make([]musicSearchDirectory, 0)
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return directories
	}
	for index := 0; index < 26; index++ {
		if mask&(1<<index) == 0 {
			continue
		}
		root := string(rune('A'+index)) + `:\`
		rootPtr, pointerErr := windows.UTF16PtrFromString(root)
		if pointerErr != nil || windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		for _, name := range names {
			directories = append(directories, musicSearchDirectory{
				Path:      filepath.Join(root, name),
				Recursive: true,
			})
		}
	}
	return directories
}

func musicDirectoriesFromConfigRoots(roots []string) []musicSearchDirectory {
	directories := make([]musicSearchDirectory, 0)
	seenDirectories := make(map[string]struct{})
	filesRead := 0
	for _, root := range roots {
		if filesRead >= maxMusicConfigFiles {
			break
		}
		rootInfo, err := os.Stat(root)
		if err != nil || !rootInfo.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if filesRead >= maxMusicConfigFiles {
				return filepath.SkipAll
			}
			if walkErr != nil {
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			relativePath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			depth := strings.Count(relativePath, string(filepath.Separator))
			if entry.IsDir() {
				if relativePath != "." && (depth >= maxMusicConfigDepth || skipMusicConfigDirectory(entry.Name())) {
					return filepath.SkipDir
				}
				return nil
			}
			if depth > maxMusicConfigDepth || !isMusicConfigCandidate(entry.Name()) {
				return nil
			}
			info, infoErr := entry.Info()
			if infoErr != nil || info.Size() <= 0 || info.Size() > maxMusicConfigFileSize {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			filesRead++
			for _, candidate := range extractWindowsAbsolutePaths(data) {
				info, statErr := os.Stat(candidate)
				if statErr != nil {
					continue
				}
				directory := candidate
				if !info.IsDir() {
					directory = filepath.Dir(candidate)
				}
				directory = filepath.Clean(directory)
				key := strings.ToLower(directory)
				if _, exists := seenDirectories[key]; exists {
					continue
				}
				seenDirectories[key] = struct{}{}
				directories = append(directories, musicSearchDirectory{Path: directory, Recursive: true})
			}
			return nil
		})
	}
	return directories
}

func isMusicConfigCandidate(name string) bool {
	lowerName := strings.ToLower(name)
	if lowerName == "localdata" || lowerName == "localware" || lowerName == "library.dat" {
		return true
	}
	switch strings.ToLower(filepath.Ext(lowerName)) {
	case ".ini", ".json", ".xml", ".cfg", ".conf", ".config":
		return true
	default:
		return false
	}
}

func skipMusicConfigDirectory(name string) bool {
	switch strings.ToLower(name) {
	case "cache", "gpucache", "log", "logs", "nim", "statics", "temp", "tmp", "webapp91x64":
		return true
	default:
		return false
	}
}

func extractWindowsAbsolutePaths(data []byte) []string {
	contents := []string{string(data)}
	if len(data) >= 4 {
		for offset := 0; offset < 2; offset++ {
			units := make([]uint16, 0, (len(data)-offset)/2)
			for index := offset; index+1 < len(data); index += 2 {
				units = append(units, binary.LittleEndian.Uint16(data[index:index+2]))
			}
			contents = append(contents, string(utf16.Decode(units)))
		}
	}

	paths := make([]string, 0)
	seen := make(map[string]struct{})
	for _, content := range contents {
		for _, match := range windowsAbsolutePathPattern.FindAllString(content, -1) {
			candidate := strings.TrimSpace(strings.TrimRight(match, " ,;)}]"))
			for strings.Contains(candidate, `\\`) {
				candidate = strings.ReplaceAll(candidate, `\\`, `\`)
			}
			candidate = filepath.Clean(candidate)
			if !filepath.IsAbs(candidate) || strings.HasPrefix(candidate, `\\`) {
				continue
			}
			key := strings.ToLower(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			paths = append(paths, candidate)
		}
	}
	return paths
}

func windowsMusicRoots() []string {
	roots := make([]string, 0, 3)
	if key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, registry.QUERY_VALUE); err == nil {
		if musicPath, _, valueErr := key.GetStringValue("My Music"); valueErr == nil && strings.TrimSpace(musicPath) != "" {
			roots = append(roots, expandWindowsEnvironment(musicPath))
		}
		_ = key.Close()
	}
	if userProfile := strings.TrimSpace(os.Getenv("USERPROFILE")); userProfile != "" {
		roots = append(roots, filepath.Join(userProfile, "Music"))
	}
	if publicProfile := strings.TrimSpace(os.Getenv("PUBLIC")); publicProfile != "" {
		roots = append(roots, filepath.Join(publicProfile, "Music"))
	}
	result := make([]string, 0, len(roots))
	seen := make(map[string]struct{})
	for _, root := range roots {
		cleanRoot := filepath.Clean(strings.TrimSpace(root))
		if !filepath.IsAbs(cleanRoot) {
			continue
		}
		key := strings.ToLower(cleanRoot)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleanRoot)
	}
	return result
}

func expandWindowsEnvironment(value string) string {
	result := strings.TrimSpace(value)
	for offset := 0; offset < len(result); {
		start := strings.Index(result[offset:], "%")
		if start < 0 {
			break
		}
		start += offset
		end := strings.Index(result[start+1:], "%")
		if end < 0 {
			break
		}
		end += start + 1
		name := result[start+1 : end]
		replacement, found := os.LookupEnv(name)
		if !found {
			offset = end + 1
			continue
		}
		result = result[:start] + replacement + result[end+1:]
		offset = start + len(replacement)
	}
	return result
}

func musicDirectoriesUnder(roots []string, names ...string) []musicSearchDirectory {
	directories := make([]musicSearchDirectory, 0, len(roots)*len(names))
	for _, root := range roots {
		for _, name := range names {
			directories = append(directories, musicSearchDirectory{
				Path:      filepath.Join(root, name),
				Recursive: true,
			})
		}
	}
	return directories
}

func kugouConfigPaths() []string {
	paths := make([]string, 0, 3)
	if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
		paths = append(paths,
			filepath.Join(appData, "KuGou8", "KuGou.ini"),
			filepath.Join(appData, "KuGou", "KuGou.ini"),
		)
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, `Software\KuGou`, registry.QUERY_VALUE); err == nil {
		if appDataPath, _, valueErr := key.GetStringValue("AppDataPath"); valueErr == nil && strings.TrimSpace(appDataPath) != "" {
			paths = append(paths, filepath.Join(strings.TrimSpace(appDataPath), "KuGou.ini"))
		}
		_ = key.Close()
	}

	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{})
	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		key := strings.ToLower(cleanPath)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleanPath)
	}
	return result
}

func readMusicINIValue(path string, wantedSection string, wantedKey string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := decodeMusicConfig(data)
	section := ""
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if !strings.EqualFold(section, wantedSection) {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && strings.EqualFold(strings.TrimSpace(key), wantedKey) {
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return ""
}

func decodeMusicConfig(data []byte) string {
	if len(data) < 2 {
		return string(data)
	}
	var order binary.ByteOrder
	switch {
	case data[0] == 0xff && data[1] == 0xfe:
		order = binary.LittleEndian
	case data[0] == 0xfe && data[1] == 0xff:
		order = binary.BigEndian
	default:
		return strings.TrimPrefix(string(data), "\ufeff")
	}
	data = data[2:]
	units := make([]uint16, 0, len(data)/2)
	for len(data) >= 2 {
		units = append(units, order.Uint16(data[:2]))
		data = data[2:]
	}
	return string(utf16.Decode(units))
}
