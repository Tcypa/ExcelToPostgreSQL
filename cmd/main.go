package main

import (
	"flag"
	"log"
	"time"
	"xlsxtoSQL/config"
	"xlsxtoSQL/processXlsx"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the config file")
	flag.Parse()

	err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg := config.GetConfig()
	for {
		for _, file := range cfg.ExcelFilePaths {
			err := processXlsx.ProcessExcelFile(*cfg, file)
			if err != nil {
				log.Printf("Error processing excel file: %v", err)
			}
		}

		time.Sleep(time.Duration(cfg.IntervalSeconds) * time.Second)
	}
}
