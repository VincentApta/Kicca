// kicca api server.
package main

import (
	"log"

	"github.com/VincentApta/Kicca/api/internal/config"
	"github.com/VincentApta/Kicca/api/internal/db"
	"github.com/VincentApta/Kicca/api/internal/github"
	"github.com/VincentApta/Kicca/api/internal/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	gdb, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := db.SeedAdmin(gdb, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("seed: %v", err)
	}

	ghEncKey, err := github.ParseEncKey(cfg.GHEncKey)
	if err != nil {
		log.Fatalf("GH_ENC_KEY: %v", err)
	}

	app := fiber.New()
	app.Use(recover.New())
	handlers.Register(app, gdb, cfg.JWTSecret, ghEncKey)

	log.Printf("kicca api listening on :%s (%s)", cfg.Port, cfg)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
