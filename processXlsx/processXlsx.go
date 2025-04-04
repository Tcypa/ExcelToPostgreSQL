package processXlsx

import (
	"context"
	"fmt"
	"log"
	"strings"
	cfg "xlsxtoSQL/config"
	"xlsxtoSQL/datatype"
	"xlsxtoSQL/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
	"github.com/xuri/excelize/v2"
)

func ProcessExcelFile(config cfg.Config, file string) error {
	xlsx, err := excelize.OpenFile(file)
	if err != nil {
		return fmt.Errorf("failed to open XLSX file: %w", err)
	}
	defer xlsx.Close()

	ctx := context.Background()
	p := postgres.Init(ctx)
	defer p.Close()

	schema := strings.ReplaceAll(file, " ", "_")

	if err := createSchema(p.Pool, schema); err != nil {
		return err
	}

	sheets := xlsx.GetSheetMap()
	for _, sheetName := range sheets {
		if sheetName == "" {
			continue
		}
		if contains(config.IgnorantSheets, sheetName) {
			log.Printf("sheet %s in ignorant list", sheetName)
			continue
		}
		createAndInsert(ctx, p.Pool, xlsx, sheetName, schema)
	}
	return nil
}

func createSchema(conn *pgxpool.Pool, schema string) error {
	ctx := context.Background()

	var exists bool
	query := "SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1);"
	err := conn.QueryRow(ctx, query, schema).Scan(&exists)
	if err != nil {
		log.Printf("error check schema exist %s: %v", schema, err)
		return err
	}

	if !exists {
		createQuery := fmt.Sprintf("CREATE SCHEMA %s;", pq.QuoteIdentifier(schema))
		_, err := conn.Exec(ctx, createQuery)
		if err != nil {
			return fmt.Errorf("error on create schema %s: %v", schema, err)
		}
		log.Printf("schema %s created", schema)
	} else {
		log.Printf("schema %s already exists", schema)
	}
	return nil
}

func createAndInsert(ctx context.Context, dbPool *pgxpool.Pool, xlsx *excelize.File, sheetName, schema string) {
	rows, err := xlsx.GetRows(sheetName)
	if err != nil {
		log.Printf("error while get rows from xlsx file sheet: %s err: %v ", sheetName, err)
	}
	if len(rows) < 2 {
		log.Printf("sheet %s is empty or has an invalid header row", sheetName)
		return
	}

	headerRow := rows[0]
	dataRows := rows[1:]

	columnTypes := datatype.DetectColumnTypes(dataRows)

	createTable(ctx, dbPool, schema, sheetName, headerRow, columnTypes)

	for rowIndex, row := range dataRows {
		insertRow(ctx, dbPool, sheetName, headerRow, row, rowIndex+1, schema, columnTypes)
	}
	log.Printf("Data inserted or updated in table %s successfully", sheetName)
}

func createTable(ctx context.Context, dbPool *pgxpool.Pool, schema, sheetName string, columns, columnTypes []string) {
	var schemaBuilder strings.Builder
	schemaBuilder.WriteString(fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (\n",
		pq.QuoteIdentifier(schema),
		pq.QuoteIdentifier(sheetName),
	))
	schemaBuilder.WriteString("id_row SERIAL PRIMARY KEY,\n")

	for i, column := range columns {
		if column == "" {
			continue
		}

		schemaBuilder.WriteString(fmt.Sprintf("%s %s", pq.QuoteIdentifier(column), columnTypes[i]))
		if i < len(columns)-1 {
			schemaBuilder.WriteString(",\n")
		}
	}

	schemaSQL := strings.TrimSuffix(schemaBuilder.String(), ",\n") + ");"
	_, err := dbPool.Exec(ctx, schemaSQL)
	if err != nil {
		log.Fatalf("failed to create table %s: %v", sheetName, err)
	}
	log.Printf("Table %s created successfully", sheetName)
}

func insertRow(ctx context.Context, dbPool *pgxpool.Pool, tableName string, columns, row []string, rowIndex int, schema string, columnTypes []string) {
	insertValues := make([]interface{}, 0)
	insertValues = append(insertValues, rowIndex)
	placeholders := []string{"$1"}


	if len(row) < len(columns) {
		padded := make([]string, len(columns))
		copy(padded, row)
		row = padded
	}


	for i, column := range columns {
		if column == "" {
			continue
		}
		value := row[i]

		if columnTypes[i] == "DATE" && strings.TrimSpace(value) != "" {
			converted, err := datatype.ConvertToDate(value)
			if err != nil {
				log.Printf("failed to convert date value '%s' in column %s: %v", value, column, err)
			} else {
				value = converted
			}
		}
		insertValues = append(insertValues, value)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(insertValues)))
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s.%s (id_row, %s) VALUES (%s) ON CONFLICT (id_row) DO UPDATE SET %s",
		pq.QuoteIdentifier(schema),
		pq.QuoteIdentifier(tableName),
		strings.Join(quoteIdentifiers(columns), ", "),
		strings.Join(placeholders, ", "),
		buildUpdateSetClause(columns),
	)

	_, err := dbPool.Exec(ctx, insertQuery, insertValues...)
	if err != nil {
		log.Printf("error while inserting row %d in table %s: %v", rowIndex, tableName, err)
		if pqErr, ok := err.(*pq.Error); ok {
			log.Printf("PostgreSQL error: %s", pqErr.Code)
			adjustColumnType(ctx, dbPool, schema, tableName, columns, columnTypes, row)
		}
	}
}

func buildUpdateSetClause(columns []string) string {
	var sets []string
	for _, col := range columns {
		sets = append(sets, fmt.Sprintf("%s = EXCLUDED.%s", pq.QuoteIdentifier(col), pq.QuoteIdentifier(col)))
	}
	return strings.Join(sets, ", ")
}

func adjustColumnType(ctx context.Context, dbPool *pgxpool.Pool, schema, tableName string, columns, columnTypes, row []string) {
	for i, column := range columns {
		if column == "" {
			continue
		}
		newType := datatype.DetermineType(row[i])
		if newType != columnTypes[i] {
			alterQuery := fmt.Sprintf(
				"ALTER TABLE %s.%s ALTER COLUMN %s SET DATA TYPE %s USING %s::%s",
				pq.QuoteIdentifier(schema),
				pq.QuoteIdentifier(tableName),
				pq.QuoteIdentifier(column),
				newType,
				pq.QuoteIdentifier(column),
				newType,
			)
			_, err := dbPool.Exec(ctx, alterQuery)
			if err != nil {
				log.Printf("failed to alter column %s in table %s: %v", column, tableName, err)
			} else {
				log.Printf("Column %s in table %s changed to %s", column, tableName, newType)
				columnTypes[i] = newType
			}
		}
	}
}
