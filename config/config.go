package config

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ExcelFilePaths    []string `yaml:"excel_file_paths"`
	PostgresURLBaseDB string   `yaml:"postgres_url_base_db"`
	IntervalSeconds   int      `yaml:"interval_seconds"`
	IgnorantSheets    []string `yaml:"ignorant_sheets"`
}

var (
	config *Config
	once   sync.Once
	err    error
)

func LoadConfig(filename string) error {
	once.Do(func() {
		var cfg Config
		file, openErr := os.Open(filename)
		if openErr != nil {
			err = fmt.Errorf("failed to open config file: %w", openErr)
			return
		}
		defer file.Close()

		decoder := yaml.NewDecoder(file)
		if decodeErr := decoder.Decode(&cfg); decodeErr != nil {
			err = fmt.Errorf("failed to decode config file: %w", decodeErr)
			return
		}

		config = &cfg
	})

	return err
}

func GetConfig() *Config {
	if config == nil {
		log.Fatal("Config not initialized. Call LoadConfig() first.")
	}
	return config
}
