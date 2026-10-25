package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/catalog/importer"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/adaptive-dsa/platform/internal/platform/logger"
)

func main() {
	filePath := flag.String("file", "", "Path to DSA question bank Excel spreadsheet (.xlsx)")
	dryRun := flag.Bool("dry-run", false, "Preview and validate questions without writing to the database")
	commit := flag.Bool("commit", false, "Execute the atomic import transaction to database")
	confirmProd := flag.Bool("confirm-prod", false, "Explicit confirmation required to run in production")
	flag.Parse()

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: --file argument is required.")
		fmt.Fprintln(os.Stderr, "Usage: import-questions --file=<path-to-excel> [--dry-run | --commit] [--confirm-prod]")
		os.Exit(1)
	}

	if !*dryRun && !*commit {
		fmt.Fprintln(os.Stderr, "Error: Must specify either --dry-run or --commit.")
		fmt.Fprintln(os.Stderr, "Tip: Run with --dry-run first to validate your spreadsheet.")
		os.Exit(1)
	}

	if os.Getenv("DATABASE_URL") == "" && *dryRun {
		_ = os.Setenv("DATABASE_URL", "postgres://offline:offline@localhost:5432/offline?sslmode=disable")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format, os.Stdout)

	if cfg.AppEnv == "production" && *commit && !*confirmProd {
		fmt.Fprintln(os.Stderr, "Error: Refusing to import directly into production environment without --confirm-prod safeguard flag.")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var repo catalog.Repository
	db, err := database.New(ctx, cfg.Database, log)
	if err != nil {
		if *dryRun {
			fmt.Printf("[NOTE] Live database offline. Performing offline spreadsheet contract validation.\n")
			repo = catalog.NewInMemoryRepository()
		} else {
			fmt.Fprintf(os.Stderr, "Database connection error: %v\n", err)
			os.Exit(1)
		}
	} else {
		defer db.Close()
		repo = catalog.NewPostgresRepository(db.Pool())
	}

	imp := importer.New(repo)

	opts := importer.ImportOptions{
		FilePath:    *filePath,
		DryRun:      *dryRun,
		Environment: cfg.AppEnv,
		ConfirmProd: *confirmProd,
	}

	fmt.Printf("\n=== DSA QUESTION BANK IMPORTER ===\n")
	fmt.Printf("File:        %s\n", *filePath)
	fmt.Printf("Mode:        %s\n", map[bool]string{true: "DRY-RUN (Validation Only)", false: "ATOMIC COMMIT"}[*dryRun])
	fmt.Printf("Environment: %s\n\n", cfg.AppEnv)

	res, err := imp.Execute(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Import failed: %v\n\n", err)
		if res != nil && len(res.RowErrors) > 0 {
			fmt.Printf("Validation Errors (%d):\n", len(res.RowErrors))
			for _, rErr := range res.RowErrors {
				fmt.Printf("  - %s\n", rErr.Error())
			}
		}
		os.Exit(1)
	}

	fmt.Println("=== IMPORT REPORT ===")
	fmt.Printf("Total Rows Read:      %d\n", res.TotalRowsRead)
	fmt.Printf("Valid Rows:           %d\n", res.ValidRows)
	fmt.Printf("Invalid Rows:         %d\n", res.InvalidRows)
	fmt.Println("----------------------------------------")
	fmt.Printf("Questions to Create:  %d\n", res.CreatedQuestions)
	fmt.Printf("Questions to Update:  %d\n", res.UpdatedQuestions)
	fmt.Printf("Questions Unchanged:  %d\n", res.UnchangedQuestions)
	fmt.Printf("Topics Created:       %d\n", res.CreatedTopics)
	fmt.Printf("Patterns Created:     %d\n", res.CreatedPatterns)
	fmt.Printf("Companies Created:    %d\n", res.CreatedCompanies)
	fmt.Println("----------------------------------------")

	if len(res.RowErrors) > 0 {
		fmt.Printf("\nRow-Level Errors Encountered (%d):\n", len(res.RowErrors))
		for _, rErr := range res.RowErrors {
			fmt.Printf("  [!] %s\n", rErr.Error())
		}
	}

	if *dryRun {
		fmt.Println("\nResult: DRY-RUN SUCCESSFUL. Zero changes made to database.")
		fmt.Println("Run with --commit to apply these changes.")
	} else {
		fmt.Println("\nResult: ATOMIC COMMIT SUCCESSFUL. Database updated.")
	}
}
