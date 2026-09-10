package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("CIVORA_TEST_DB_USER"), os.Getenv("CIVORA_TEST_DB_PASSWORD"),
		os.Getenv("CIVORA_TEST_DB_HOST"), os.Getenv("CIVORA_TEST_DB_PORT"), os.Getenv("CIVORA_TEST_DB_NAME"))
	db, err := database.NewDatabase(dsn, "pgx")
	if err != nil {
		fmt.Println("connect error:", err)
		return
	}
	defer db.DB.Close()

	ctx := context.Background()
	migrator := database.NewMigrator(db.DB, migrations.FS)
	if err := migrator.LoadMigrations(); err != nil {
		fmt.Println("load error:", err)
		return
	}

	if err := migrator.Migrate(ctx); err != nil {
		fmt.Println("migrate error:", err)
		return
	}

	rows, err := db.DB.QueryContext(ctx, "SELECT version, name FROM schema_migrations ORDER BY version")
	if err != nil {
		fmt.Println("cols error:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		var n string
		rows.Scan(&v, &n)
		fmt.Printf("applied: v%d %s\n", v, n)
	}

	var cols []string
	rows2, err := db.DB.QueryContext(ctx, "SELECT column_name FROM information_schema.columns WHERE table_name='cases' ORDER BY ordinal_position")
	if err != nil {
		fmt.Println("cols error:", err)
		return
	}
	defer rows2.Close()
	for rows2.Next() {
		var c string
		rows2.Scan(&c)
		cols = append(cols, c)
	}
	fmt.Println("cases columns:", cols)
}