// kicca api server. T1: health endpoint + env config only.
package main

import (
	"log"

	"github.com/VincentApta/Kicca/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	app := fiber.New()
	app.Use(recover.New())

	// Static routes must be registered before any parametric ones
	// (e.g. /api/health before /api/:resource) so Fiber matches them first.
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	log.Printf("kicca api listening on :%s (%s)", cfg.Port, cfg)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
