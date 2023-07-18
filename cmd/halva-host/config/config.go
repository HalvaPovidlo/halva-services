package config

import (
	"os"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

type Config struct {
	General GeneralConfig
}

type GeneralConfig struct {
	Debug     bool
	IP        string   `yaml:"ip" split_words:"true"`
	Port      string   `yaml:"port" split_words:"true"`
	Path      string   `yaml:"path" split_words:"true"`
	Endpoints []string `yaml:"endpoints"`
	Level     zapcore.Level
}

func InitConfig(configPathEnv, envPrefix string) (Config, error) {
	var (
		configData []byte
		err        error
		filePath   string
	)

	if filePath = os.Getenv(configPathEnv); filePath == "" {
		filePath = "cmd/halva-host/config/secret.yaml"
	}

	if configData, err = os.ReadFile(filepath.Clean(filePath)); err != nil {
		return Config{}, err
	}

	expandedData := os.ExpandEnv(string(configData))

	var cfg Config
	if err := yaml.UnmarshalStrict([]byte(expandedData), &cfg); err != nil {
		return Config{}, err
	}
	if err := envconfig.Process(envPrefix, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
