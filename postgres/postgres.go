package postgres

import (
	"context"
	"log"
	"sync"
	cfg "xlsxtoSQL/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgStorage struct {
	Mu   sync.RWMutex
	Pool *pgxpool.Pool
}

func Init(ctx context.Context) *PgStorage {
	cfg := cfg.GetConfig()

	var err error
	pool, err := pgxpool.New(ctx, cfg.PostgresURLBaseDB)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}

	log.Println("Postgres сonnect success")
	return &PgStorage{Pool: pool}
}
func (p *PgStorage) Close() {
	if p.Pool != nil {
		p.Pool.Close()
		log.Println("Database connect closed")
	}
}
