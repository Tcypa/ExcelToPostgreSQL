package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ExcelFilePaths    []string `yaml:"excel_file_paths"`
	PostgresURLBaseDB string   `yaml:"postgres_url_base_db"`
	IntervalSeconds   int      `yaml:"interval_seconds"`
	IgnorantSheets    []string `yaml:"ignorant_sheets"`
}

func LoadConfig(filename string) (Config, error) {
	var config Config
	file, err := os.Open(filename)
	if err != nil {
		return config, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("failed to decode config file: %w", err)
	}

	return config, nil
}
