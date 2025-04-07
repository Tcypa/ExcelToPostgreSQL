package main

import (
	"flag"
	"log"
	"os"
	"time"
	"xlsxtoSQL/config"
	"xlsxtoSQL/processXlsx"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the config file")
	once := flag.Bool("once", false, "run once and exit")
	flag.Parse()

	err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg := config.GetConfig()

	if *once {
		for _, file := range cfg.ExcelFilePaths {
			err := processXlsx.ProcessExcelFile(*cfg, file)
			if err != nil {
				log.Printf("Error processing excel file: %v", err)
			}
		}
		os.Exit(0)
	} else {
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
}
