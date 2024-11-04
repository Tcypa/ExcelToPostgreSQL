package main

import (
	"log"
	"runtime"
	"time"

	"xlsxtoSQL/cmd/converter/cfg"
	"xlsxtoSQL/cmd/converter/processXlsx"
)

func main() {
	config, err := cfg.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	for {
		for _, file := range config.ExcelFilePaths {
			err := processXlsx.ProcessExcelFile(config, file)
			if err != nil {
				log.Printf("error processing excel file: %v", err)
			}
		}
		runtime.GC()

		time.Sleep(time.Duration(config.IntervalSeconds) * time.Second)
	}
}
