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
  create    Provision a new administrator account with RoleAdmin (interactively prompts for password)

Flags for create:
  --email string
        Administrator email address (required)
  --name string
        Administrator full name (required)`)
}
