package processXlsx

import (

	"github.com/lib/pq"
)

func quoteIdentifiers(names []string) []string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" {
			quoted = append(quoted, pq.QuoteIdentifier(name))
		}
	}
	return quoted
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
