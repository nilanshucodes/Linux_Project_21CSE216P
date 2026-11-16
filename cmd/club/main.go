package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"student-club-system/internal/cli"
	"student-club-system/internal/member"
	"student-club-system/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPath := os.Getenv("CLUB_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/club.db"
	}

	store, err := storage.Open(dbPath)
	if err != nil {
		fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close database: %v\n", err)
		}
	}()

	seeded, err := store.SeedIfEmpty(ctx)
	if err != nil {
		fatal(err)
	}
	if seeded {
		fmt.Println("Loaded fictional sample members into the empty database.")
	}

	service := member.NewService(store)
	app := cli.NewApp(service, os.Stdin, os.Stdout)
	if err := app.Run(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Student Club Membership System: %v\n", err)
	os.Exit(1)
}
