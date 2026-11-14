package config

import (
	"errors"
	"io/fs"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Env string

const (
	Prod Env = "prod"
	Dev  Env = "dev"
)

type Config struct {
	DBURL      string
	Port       string
	AdminUser  string
	AdminPass  string
	ViewerUser string
	ViewerPass string
}

func Load() *Config {
	// .env is for local development. In the container, the variables come
	// from the environment (Dokploy) and there is no .env file.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatal("Error loading .env file: ", err)
	}

	cfg := &Config{
		DBURL:      os.Getenv("POSTGRESURL"),
		Port:       os.Getenv("PORT"),
		AdminUser:  os.Getenv("ADMIN_USER"),
		AdminPass:  os.Getenv("ADMIN_PASS"),
		ViewerUser: os.Getenv("VIEWER_USER"),
		ViewerPass: os.Getenv("VIEWER_PASS"),
	}

	for name, v := range map[string]string{
		"ADMIN_USER":  cfg.AdminUser,
		"ADMIN_PASS":  cfg.AdminPass,
		"VIEWER_USER": cfg.ViewerUser,
		"VIEWER_PASS": cfg.ViewerPass,
	} {
		if v == "" {
			log.Fatalf("Missing required env var %s", name)
		}
	}
	if cfg.AdminUser == cfg.ViewerUser {
		log.Fatal("ADMIN_USER and VIEWER_USER must be different")
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg
}
