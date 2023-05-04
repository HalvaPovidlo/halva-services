package config

import (
	"os"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"

	"github.com/HalvaPovidlo/halva-services/internal/halva-auth-api/auth"
)

type Config struct {
	General GeneralConfig
	Login   auth.Config
}

type GeneralConfig struct {
	Debug  bool
	Host   string `yaml:"host" split_words:"true"`
	Port   string `yaml:"port" split_words:"true"`
	Secret string `yaml:"secret" split_words:"true"`
	Level  zapcore.Level
}

func InitConfig(filePath, envPrefix string) (Config, error) {
	var (
		configData []byte
		err        error
	)
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
