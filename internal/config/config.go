package config

import (
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
	DBURL string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	DBURL := os.Getenv("POSTGRESURL")

	return &Config{
		DBURL: DBURL,
	}
}
