package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/adaptive-dsa/platform/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationsDir = "migrations"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var dir string
	var confirmReset bool
	flag.StringVar(&dir, "dir", migrationsDir, "directory containing migration files")
	flag.BoolVar(&confirmReset, "confirm-danger-reset", false, "explicit confirmation required to roll back all migrations")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	command := strings.ToLower(args[0])

	// For create command, no DB connection is needed
	if command == "create" {
		if len(args) < 2 {
			return fmt.Errorf("create requires migration name: migrate create <name> [sql|go]")
		}
		name := args[1]
		migrationType := "sql"
		if len(args) > 2 {
			migrationType = args[2]
		}
		return goose.Create(nil, dir, name, migrationType)
	}

	// Load database configuration safely
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration failed: %w", err)
	}

	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to open database connection (%s): %w", cfg.Database.RedactedDatabaseURL(), err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect to postgres: %w", err)
	}

	fmt.Printf("Connected to database: %s (environment: %s)\n", cfg.Database.RedactedDatabaseURL(), cfg.AppEnv)
	fmt.Printf("Executing migration command '%s' in directory '%s'...\n", command, dir)

	switch command {
	case "up":
		return goose.Up(db, dir)
	case "up-by-one":
		return goose.UpByOne(db, dir)
	case "down":
		return goose.Down(db, dir)
	case "status":
		return goose.Status(db, dir)
	case "version":
		return goose.Version(db, dir)
	case "reset":
		if cfg.AppEnv == config.EnvProduction {
			return fmt.Errorf("dangerous operation aborted: reset is strictly forbidden when APP_ENV is 'production'")
		}
		if !confirmReset {
			return fmt.Errorf("destructive operation aborted: reset rolls back ALL migrations and destroys data. Pass -confirm-danger-reset to confirm")
		}
		return goose.Reset(db, dir)
	default:
		printUsage()
		return fmt.Errorf("unknown command '%s'", command)
	}
}

func printUsage() {
	fmt.Println(`Goose Database Migration Runner

Usage:
  go run cmd/migrate/main.go [flags] <command> [arguments]

Commands:
  up         Migrate the DB to the most recent version available
  up-by-one  Migrate the DB up by 1 version
  down       Roll back the version by 1
  status     Dump the migration status for the current DB
  version    Print the current version of the database
  create     Create a new migration file: create <name> [sql|go]
  reset      Roll back all migrations

Flags:
  -dir string
        directory containing migration files (default "migrations")
  -confirm-danger-reset
        explicit confirmation required to execute 'reset' (cannot be run in production)`)
}
