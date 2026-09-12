// kica api server.
package main

import (
	"log"

	"github.com/VincentApta/Kica/api/internal/config"
	"github.com/VincentApta/Kica/api/internal/db"
	"github.com/VincentApta/Kica/api/internal/github"
	"github.com/VincentApta/Kica/api/internal/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	gdb, migrations, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := db.SeedAdmin(gdb, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("seed: %v", err)
	}
	if err := db.SeedClient(gdb, cfg.ClientEmail, cfg.ClientPassword); err != nil {
		log.Fatalf("seed client: %v", err)
	}

	// validation already happened in config.Load; this just decodes the bytes
	ghEncKey, _ := github.ParseEncKey(cfg.GHEncKey)

	app := fiber.New()
	app.Use(recover.New())
	handlers.Register(app, gdb, cfg.JWTSecret, ghEncKey)

	log.Printf("kica api listening on :%s (migrations applied: %d, %s)", cfg.Port, migrations, cfg)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
