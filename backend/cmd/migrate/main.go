// Command migrate применяет/откатывает миграции БД через goose.
//
// Использование:
//
//	migrate up            — применить все миграции
//	migrate down          — откатить одну миграцию
//	migrate status        — показать состояние
//
// Каталог миграций по умолчанию — ./migrations, переопределяется флагом -dir.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib" // драйвер database/sql на базе pgx
	"github.com/pressly/goose/v3"

	"github.com/healthpage/backend/internal/config"
)

func main() {
	dir := flag.String("dir", "migrations", "каталог с файлами миграций")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("usage: migrate [-dir <path>] <up|down|status|...> [args]")
	}
	command := args[0]

	cfg := config.Load()
	db, err := sql.Open("pgx", cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set dialect: %v", err)
	}

	if err := goose.RunContext(context.Background(), command, db, *dir, args[1:]...); err != nil {
		log.Fatalf("goose %s: %v", command, err)
	}
}
