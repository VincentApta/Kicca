// Package db opens the Postgres connection, runs golang-migrate SQL
// migrations, and seeds the first-boot admin.
package db

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/VincentApta/Kica/api/internal/models"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open connects via DATABASE_URL (gorm) and applies pending migrations.
// Returns the applied-migration count (== the sequential schema version;
// golang-migrate stores only the current one).
func Open(databaseURL string) (*gorm.DB, int, error) {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, 0, fmt.Errorf("migrations fs: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, databaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, 0, fmt.Errorf("migrate up: %w", err)
	}
	version, _, err := m.Version()
	if err != nil {
		return nil, 0, fmt.Errorf("migrate version: %w", err)
	}
	m.Close()

	gdb, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, 0, fmt.Errorf("gorm open: %w", err)
	}
	return gdb, int(version), nil
}

// SeedAdmin creates the initial admin from ADMIN_EMAIL/ADMIN_PASSWORD iff the
// users table is empty. Idempotent: any existing user skips the seed.
func SeedAdmin(gdb *gorm.DB, email, password string) error {
	if email == "" || password == "" {
		log.Printf("seed: ADMIN_EMAIL/ADMIN_PASSWORD not set, skipping")
		return nil
	}
	var count int64
	if err := gdb.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := &models.User{
		Email:        strings.ToLower(email),
		PasswordHash: string(hash),
		Name:         strings.SplitN(email, "@", 2)[0],
		GlobalRole:   "admin",
	}
	if err := gdb.Create(admin).Error; err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	log.Printf("seed: created admin %s", admin.Email)
	return nil
}
