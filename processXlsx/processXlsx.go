package processXlsx

import (
	"context"
	"fmt"
	"log"
	"strings"
	cfg "xlsxtoSQL/config"
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

	err = createSchema(p.Pool, schema)
	if err != nil {
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
		log.Printf("error check scheme exist %s: %v", schema, err)
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

	var tableExists bool
	exst := "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2);"
	err = dbPool.QueryRow(ctx, exst, schema, sheetName).Scan(&tableExists)
	if err != nil {
		log.Fatalf("failed to check if table exists: %v", err)
	}

	if !tableExists {
		var schemaBuilder strings.Builder
		schemaBuilder.WriteString(fmt.Sprintf(
			"CREATE TABLE %s.%s (\n",
			pq.QuoteIdentifier(schema),
			pq.QuoteIdentifier(sheetName),
		))
		schemaBuilder.WriteString("id_row SERIAL PRIMARY KEY,\n")
		for i, column := range headerRow {
			if column == "" {
				continue
			}
			schemaBuilder.WriteString(fmt.Sprintf("%s TEXT", pq.QuoteIdentifier(column)))
			if i < len(headerRow)-1 {
				schemaBuilder.WriteString(",\n")
			}
		}
		schemaSQL := strings.TrimSuffix(schemaBuilder.String(), ",\n") + ");"
		_, err = dbPool.Exec(ctx, schemaSQL)
		if err != nil {
			log.Fatalf("failed to create table %s: %v", sheetName, err)
		}
		log.Printf("Table %s created successfully", sheetName)
	}

	rowIndex := 1
	for rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			log.Printf("failed to read row %d in sheet %s: %v", rowIndex, sheetName, err)
			continue
		}
		insertRow(ctx, dbPool, sheetName, headerRow, row, rowIndex, schema)
		rowIndex++
	}
	log.Printf("Data inserted or updated in table %s successfully", sheetName)
}

func insertRow(ctx context.Context, dbPool *pgxpool.Pool, tableName string, columns, row []string, rowIndex int, schema string) {
	insertValues := make([]interface{}, 0)
	insertValues = append(insertValues, rowIndex)
	placeholders := []string{"$1"}
	for i, column := range columns {
		if column == "" {
			continue
		}
		insertValues = append(insertValues, row[i])
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(insertValues)))
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s.%s (id_row, %s) VALUES (%s)",
		pq.QuoteIdentifier(schema),
		pq.QuoteIdentifier(tableName),
		strings.Join(quoteIdentifiers(columns), ", "),
		strings.Join(placeholders, ", "),
	)
	_, err := dbPool.Exec(ctx, insertQuery, insertValues...)
	if err != nil {
		log.Printf("error while insert or update on %d in table %s: %v", rowIndex, tableName, err)
	}
}
