package processXlsx

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"xlsxtoSQL/cmd/converter/cfg"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lib/pq"
	"github.com/xuri/excelize/v2"
)

func ProcessExcelFile(config cfg.Config, file string) error {
	xlsx, err := excelize.OpenFile(file)
	if err != nil {
		return fmt.Errorf("failed to open XLSX file: %w", err)
	}
	defer func() {
		if err := xlsx.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	ctx := context.Background()

	dbPool, err := pgxpool.Connect(ctx, config.PostgresURLBaseDB)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbPool.Close()

	schema := filepath.Base(file)
	ext := filepath.Ext(schema)
	schema = strings.TrimSuffix(schema, ext)
	schema = strings.ReplaceAll(schema, " ", "_")

	err = createSchemaIfNotExists(dbPool, schema)

	sheets := xlsx.GetSheetMap()
	fmt.Println("Sheets in XLSX file:")
	for _, sheetName := range sheets {
		if sheetName == "" {
			continue
		}
		fmt.Println("-", sheetName)
		if contains(config.IgnorantSheets, sheetName) {
			log.Printf("sheet %s in ignorant list", sheetName)
			continue
		}
		doCreateAndInsert(ctx, dbPool, xlsx, sheetName, schema)
	}

	fmt.Println("Schema generation and data insertion complete.")
	return nil
}

func doCreateAndInsert(ctx context.Context, dbPool *pgxpool.Pool, xlsx *excelize.File, sheetName string, schema string) {
	rows, err := xlsx.Rows(sheetName)
	if err != nil {
		log.Printf("failed to get rows from %s: %v", sheetName, err)
		return
	}
	defer rows.Close()

	if !rows.Next() {
		log.Printf("sheet %s is empty or has an invalid header row", sheetName)
		return
	}

	headerRow, err := rows.Columns()
	if err != nil || len(headerRow) == 0 {
		log.Printf("sheet %s has an invalid header row", sheetName)
		return
	}

	columnTypes := detectColumnTypes(headerRow, xlsx, sheetName)

	var tableExists bool
	exst := fmt.Sprintf("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $$%s$$ and table_schema = $$%s$$)", sheetName, schema)
	err = dbPool.QueryRow(ctx, exst).Scan(&tableExists)
	if err != nil {
		log.Fatalf("failed to check if table exists: %v", err)
	}

	if !tableExists {
		var schemaBuilder strings.Builder
		schemaBuilder.WriteString(fmt.Sprintf("CREATE TABLE %s.%s (\n", schema, pq.QuoteIdentifier(sheetName)))
		schemaBuilder.WriteString("id_row SERIAL PRIMARY KEY,\n")

		for i, column := range headerRow {
			if column == "" {
				continue
			}
			sqlType := columnTypes[i]
			schemaBuilder.WriteString(fmt.Sprintf("%s %s", pq.QuoteIdentifier(column), sqlType))
			if i < len(headerRow)-1 {
				schemaBuilder.WriteString(",\n")
			}
		}
		schema := strings.TrimSuffix(schemaBuilder.String(), ",\n") + ");"

		_, err = dbPool.Exec(ctx, schema)
		if err != nil {
			log.Fatalf("failed to create table %s: %v", sheetName, err)
		}
		log.Printf("Table %s created successfully", sheetName)
	}

	rowIndex := 1
	for {
		if !rows.Next() {
			break
		}
		row, err := rows.Columns()
		if err != nil {
			log.Printf("failed to read row %d in sheet %s: %v", rowIndex, sheetName, err)
			continue
		}
		row = padRowToHeaderLength(row, len(headerRow))
		insertOrUpdateRow(ctx, dbPool, sheetName, headerRow, columnTypes, row, rowIndex, schema)
		rowIndex++
	}

	if err = rows.Close(); err != nil {
		log.Printf("error closing rows in sheet %s: %v", sheetName, err)
	}

	log.Printf("Data inserted or updated in table %s successfully", sheetName)
}

func padRowToHeaderLength(row []string, headerLength int) []string {
	paddedRow := make([]string, headerLength)
	for i := 0; i < headerLength; i++ {
		if i < len(row) {
			paddedRow[i] = row[i]
		} else {
			paddedRow[i] = ""
		}
	}
	return paddedRow
}

func insertOrUpdateRow(ctx context.Context, dbPool *pgxpool.Pool, tableName string, columns []string, columnTypes []string, row []string, rowIndex int, schema string) {
	updateColumns := make([]string, 0)
	insertValues := make([]interface{}, 0)
	insertValues = append(insertValues, rowIndex)

	for i, column := range columns {
		if isEmpty(column) {
			continue
		}
		updateColumns = append(updateColumns, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(column), len(insertValues)+1))
		value := convertValue(row[i], columnTypes[i])
		if value == nil {
			insertValues = append(insertValues, nil)
		} else {
			insertValues = append(insertValues, value)
		}
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s.%s (id_row, %s) VALUES ($1, %s) ON CONFLICT (id_row) DO UPDATE SET %s",
		schema,
		pq.QuoteIdentifier(tableName),
		strings.Join(quoteIdentifiers(columns), ", "),
		strings.Join(makePlaceholders(len(insertValues)-1), ", "),
		strings.Join(updateColumns, ", "),
	)
	_, err := dbPool.Exec(ctx, insertQuery, insertValues...)
	if err != nil {
		log.Printf("ошибка при вставке или обновлении строки %d в таблице %s: %v", rowIndex, tableName, err)
		log.Printf("запрос: %s", insertQuery)
		log.Printf("значения: %v", insertValues)
		log.Printf("Колонки: %v", columnTypes)
	}
}

func detectColumnTypes(columns []string, xlsx *excelize.File, sheetName string) []string {
	columnTypes := make([]string, len(columns))
	rows, err := xlsx.Rows(sheetName)
	if err != nil {
		log.Printf("failed to get rows from %s: %v", sheetName, err)
	}
	defer rows.Close()
	for i := range columns {
		columnTypes[i] = "TEXT"
	}

	for rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			continue
		}
		for i, value := range row {
			//trimmedValue := strings.TrimSpace(value)
			trimmedValue := strings.ReplaceAll(value, ",", "")
			if isInt(trimmedValue) {
				columnTypes[i] = "NUMERIC"
			} else if isDate(trimmedValue) {
				columnTypes[i] = "DATE"
			} else {
				columnTypes[i] = "TEXT"
			}
		}
	}

	return columnTypes
}

func createSchemaIfNotExists(conn *pgxpool.Pool, schema string) error {

	ctx := context.Background()

	var exists bool

	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s');", schema)

	err := conn.QueryRow(ctx, query).Scan(&exists)

	if err != nil {
		log.Printf("ошибка проверки существования схемы %s: %v", schema, err)
		return nil
	}
	if !exists {
		_, err := conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s;", schema))

		if err != nil {

			return fmt.Errorf("ошибка создания схемы %s: %v", schema, err)
		}
		log.Printf("Схема %s создана", schema)
	} else {
		log.Printf("Схема %s уже существует", schema)
	}

	return nil

}
