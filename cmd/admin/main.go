package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/adaptive-dsa/platform/internal/platform/logger"
	"github.com/google/uuid"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	command := strings.ToLower(os.Args[1])
	switch command {
	case "create":
		return runCreateAdmin(os.Args[2:])
	case "reconcile-credits":
		return runReconcileCredits()
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func runCreateAdmin(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	email := fs.String("email", "", "Administrator email address (required)")
	name := fs.String("name", "", "Administrator full name (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*email) == "" || strings.TrimSpace(*name) == "" {
		printUsage()
		return fmt.Errorf("both --email and --name flags are required")
	}

	password, err := readPasswordSecurely()
	if err != nil {
		return fmt.Errorf("password input error: %w", err)
	}

	if err := identity.ValidatePassword(password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	log := logger.New(cfg.Log.Level, "text", os.Stdout)
	ctx := context.Background()

	db, err := database.New(ctx, cfg.Database, log)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	repo := identity.NewSQLRepository(db.Pool())
	svc := identity.NewService(repo, cfg, log)

	adminUser, err := svc.CreateAdmin(ctx, *email, password, *name, "127.0.0.1 (CLI)")
	if err != nil {
		return fmt.Errorf("failed to provision administrator: %w", err)
	}

	fmt.Println("==================================================")
	fmt.Println("Administrator Account Provisioned Successfully")
	fmt.Printf("User ID:    %s\n", adminUser.ID)
	fmt.Printf("Email:      %s\n", adminUser.Email)
	fmt.Printf("Full Name:  %s\n", adminUser.FullName)
	fmt.Printf("Role:       %s\n", adminUser.Role)
	fmt.Printf("Created At: %s\n", adminUser.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Println("Audit event recorded atomically in audit_logs table.")
	fmt.Println("==================================================")

	return nil
}

func readPasswordSecurely() (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Print("Enter administrator password: ")
		pass1, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}

		fmt.Print("Confirm administrator password: ")
		pass2, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read confirmation password: %w", err)
		}

		if subtle.ConstantTimeCompare(pass1, pass2) != 1 {
			zeroBytes(pass1)
			zeroBytes(pass2)
			return "", errors.New("passwords do not match")
		}

		password := string(pass1)
		zeroBytes(pass1)
		zeroBytes(pass2)
		return password, nil
	}

	// Non-terminal fallback (e.g. piped input in automated testing / scripts)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read password from stdin: %w", err)
	}
	pass := strings.TrimRight(line, "\r\n")
	if pass == "" {
		return "", errors.New("empty password provided via stdin")
	}
	return pass, nil
}

func runReconcileCredits() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	log := logger.New(cfg.Log.Level, "text", os.Stdout)
	ctx := context.Background()

	db, err := database.New(ctx, cfg.Database, log)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	rows, err := db.Pool().Query(ctx, `
		SELECT 
			a.user_id,
			u.email,
			a.cached_balance,
			COALESCE(SUM(l.amount), 0) AS ledger_sum
		FROM credit_accounts a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN credit_ledger_entries l ON l.user_id = a.user_id
		GROUP BY a.user_id, u.email, a.cached_balance
	`)
	if err != nil {
		return fmt.Errorf("failed to query credit accounts: %w", err)
	}
	defer rows.Close()

	var totalChecked int
	var discrepancies int

	fmt.Println("==================================================")
	fmt.Println("Credit Balance Reconciliation Audit")
	fmt.Println("==================================================")

	for rows.Next() {
		totalChecked++
		var userID uuid.UUID
		var email string
		var cachedBalance int32
		var ledgerSum int64

		if err := rows.Scan(&userID, &email, &cachedBalance, &ledgerSum); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if int64(cachedBalance) != ledgerSum {
			discrepancies++
			fmt.Printf("[DISCREPANCY] User: %s (%s) Cached: %d != LedgerSum: %d (Delta: %d)\n",
				email, userID, cachedBalance, ledgerSum, int64(cachedBalance)-ledgerSum)
		}
	}

	fmt.Println("--------------------------------------------------")
	if discrepancies == 0 {
		fmt.Printf("RECONCILIATION PASSED: All %d credit accounts in exact alignment with immutable ledger.\n", totalChecked)
	} else {
		fmt.Printf("RECONCILIATION FAILED: Found %d discrepancies across %d accounts!\n", discrepancies, totalChecked)
		return fmt.Errorf("credit ledger reconciliation failed with %d discrepancies", discrepancies)
	}
	fmt.Println("==================================================")
	return nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func printUsage() {
	fmt.Println(`Adaptive DSA Administrator CLI

Usage:
  go run cmd/admin/main.go <command> [flags]

Commands:
  create             Provision a new administrator account with RoleAdmin (interactively prompts for password)
  reconcile-credits  Verify double-entry credit ledger against cached balances and detect any discrepancies

Flags for create:
  --email string
        Administrator email address (required)
  --name string
        Administrator full name (required)`)
}
