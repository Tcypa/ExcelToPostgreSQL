package main

import (
	"log"
	"runtime"
	"time"

	cfg "xlsxtoSQL/config"
	"xlsxtoSQL/processXlsx"
)

func main() {

	for {
		err := cfg.LoadConfig("config.yaml")
		var config = cfg.GetConfig()
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		for _, file := range config.ExcelFilePaths {
			err := processXlsx.ProcessExcelFile(*config, file)
			if err != nil {
				log.Printf("error processing excel file: %v", err)
			}
		}
		runtime.GC()

		time.Sleep(time.Duration(config.IntervalSeconds) * time.Second)
	}
}
