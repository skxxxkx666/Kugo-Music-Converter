package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Addr            string `yaml:"addr" json:"addr"`
	FFmpegBin       string `yaml:"ffmpeg_bin" json:"ffmpeg_bin"`
	PublicDir       string `yaml:"public_dir" json:"public_dir"`
	MaxFileSize     int64  `yaml:"max_file_size" json:"max_file_size"`
	MaxFiles        int    `yaml:"max_files" json:"max_files"`
	DefaultOutput   string `yaml:"default_output" json:"default_output"`
	Concurrency     int    `yaml:"concurrency" json:"concurrency"`
	ParseFormMemory int64  `yaml:"parse_form_memory" json:"parse_form_memory"`
}

func DefaultConfig() *Config {
	return &Config{
		Addr:            ":8080",
		FFmpegBin:       "tools/ffmpeg.exe",
		PublicDir:       "public",
		MaxFileSize:     80 << 20,
		MaxFiles:        500,
		DefaultOutput:   "",
		Concurrency:     3,
		ParseFormMemory: 32 << 20,
	}
}

func LoadConfig(configPath, addr, ffmpegBin string, addrSet, ffmpegSet bool) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if env := os.Getenv("KGG_ADDR"); env != "" {
		cfg.Addr = env
	}
	if env := os.Getenv("KGG_FFMPEG_BIN"); env != "" {
		cfg.FFmpegBin = env
	}
	if env := os.Getenv("KGG_PUBLIC_DIR"); env != "" {
		cfg.PublicDir = env
	}
	if env := os.Getenv("KGG_DEFAULT_OUTPUT"); env != "" {
		cfg.DefaultOutput = env
	}
	if env := os.Getenv("KGG_MAX_FILE_SIZE"); env != "" {
		if n, err := strconv.ParseInt(env, 10, 64); err == nil && n > 0 {
			cfg.MaxFileSize = n
		}
	}
	if env := os.Getenv("KGG_MAX_FILES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.MaxFiles = n
		}
	}
	if env := os.Getenv("KGG_CONCURRENCY"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.Concurrency = n
		}
	}
	if env := os.Getenv("KGG_PARSE_FORM_MEMORY"); env != "" {
		if n, err := strconv.ParseInt(env, 10, 64); err == nil && n > 0 {
			cfg.ParseFormMemory = n
		}
	}

	if addrSet {
		cfg.Addr = addr
	}
	if ffmpegSet {
		cfg.FFmpegBin = ffmpegBin
	}

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 3
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 500
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 80 << 20
	}
	if cfg.ParseFormMemory <= 0 {
		cfg.ParseFormMemory = 32 << 20
	}
	if cfg.PublicDir == "" {
		cfg.PublicDir = "public"
	}

	return cfg, nil
}

func (c *Config) SaveExample() error {
	data, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		return err
	}
	return os.WriteFile("config.example.yaml", data, 0644)
}
