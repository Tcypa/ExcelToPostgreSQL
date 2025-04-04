package datatype

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var dateFormats = []string{
	"2006-01-02",
	"02-01-2006",
	"02.01.2006",
	"2006.01.02",
	"02/01/2006",
	"2006/01/02",
	"January 2, 2006",
	"2 January 2006",
	"02 Jan 2006",
}

func DetectColumnTypes(data [][]string) []string {
	if len(data) == 0 {
		return []string{}
	}
	numColumns := len(data[0])
	types := make([]string, numColumns)
	for col := 0; col < numColumns; col++ {
		types[col] = detectType(data, col)
	}
	return types
}

func detectType(data [][]string, col int) string {
	isInt, isFloat, isDate := true, true, true

	for _, row := range data {
		if col >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[col])
		if val == "" {
			continue
		}
		if _, err := strconv.Atoi(val); err != nil {
			isInt = false
		}
		if _, err := strconv.ParseFloat(val, 64); err != nil {
			isFloat = false
		}
		dateOk := false
		for _, format := range dateFormats {
			if _, err := time.Parse(format, val); err == nil {
				dateOk = true
				break
			}
		}
		if !dateOk {
			isDate = false
		}
	}

	switch {
	case isInt:
		return "INTEGER"
	case isFloat:
		return "FLOAT"
	case isDate:
		return "DATE"
	default:
		return "TEXT"
	}
}

func DetermineType(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return "TEXT"
	}
	if _, err := strconv.Atoi(val); err == nil {
		return "INTEGER"
	}
	if _, err := strconv.ParseFloat(val, 64); err == nil {
		return "FLOAT"
	}
	for _, format := range dateFormats {
		if _, err := time.Parse(format, val); err == nil {
			return "DATE"
		}
	}
	return "TEXT"
}
func ConvertToDate(val string) (string, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return "", nil
	}
	for _, format := range dateFormats {
		if t, err := time.Parse(format, val); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("cannot convert date: %s", val)
}
