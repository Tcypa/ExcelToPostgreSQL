package processXlsx

import (
	"fmt"
	"github.com/lib/pq"
	"strconv"
	"strings"
	"time"
)

func isInt(str string) bool {
	if _, err := strconv.Atoi(str); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(str, 64); err == nil {
		return true
	}
	return false
}

func isDate(str string) bool {
	if _, err := time.Parse("02.01.2006", str); err == nil {
		return true
	}
	if _, err := time.Parse("02-01-06", str); err == nil {
		return true
	}
	return false

}

func isEmpty(value string) bool {
	return strings.TrimSpace(value) == ""
}

func convertValue(value string, columnType string) interface{} {
	switch columnType {
	case "NUMERIC":
		if value == "" {
			return nil
		}
		value = strings.ReplaceAll(value, ",", "")
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
		return nil
	case "DATE":
		if value == "" {
			return nil
		}
		dateValue, err := time.Parse("02.01.2006", value)
		if err != nil {
			dateValue, err = time.Parse("01-02-06", value)
			if err != nil {
				return nil
			}
		}

		return dateValue.Format("2006-01-02")
	default:
		if value == "" {
			return nil
		}
		return value
	}
}

func quoteIdentifiers(columns []string) []string {
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = pq.QuoteIdentifier(column)
	}
	return quoted
}

func makePlaceholders(n int) []string {
	placeholders := make([]string, n)
	for i := 0; i < n; i++ {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
	}
	return placeholders
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
