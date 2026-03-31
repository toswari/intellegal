package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Postgres wraps a SQL connection pool used by the Go API.
type Postgres struct {
	conn *sql.DB
}

func Open(databaseURL string) (*Postgres, error) {
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	return &Postgres{conn: conn}, nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	if p == nil || p.conn == nil {
		return fmt.Errorf("postgres is not initialized")
	}
	return p.conn.PingContext(ctx)
}

func (p *Postgres) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}
	return p.conn.Close()
}

func (p *Postgres) PingWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Ping(ctx)
}

func (p *Postgres) DB() *sql.DB {
	if p == nil {
		return nil
	}
	return p.conn
}
