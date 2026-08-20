package main

import (
	"fmt"
	"os"

	"woason-api/internal/config"
	"woason-api/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "up":
		if err := db.MigrateUp(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("миграции применены")
	case "down":
		if err := db.MigrateDown(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("миграции откачены")
	default:
		fmt.Fprintln(os.Stderr, "использование: migrate [up|down]")
		os.Exit(1)
	}
}
