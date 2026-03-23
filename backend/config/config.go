package config

import (
	"errors"
	"os"

	"log"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerHost   string
	ServerPort   string
	SecretKey    string
	PostgresUrl  string
	ClientDomain string
}

// Using a singleton pattern to load the config only once and reduce read calls
var config *Config

func LoadConfig() Config {
	if config != nil {
		return *config
	}

	config = &Config{}

	err := godotenv.Load(".env")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("No .env file found, using environment variables")
		} else {
			log.Fatalf("Error loading .env file: %v", err)
		}
	} else {
		log.Println("Config loaded successfully from .env file")
	}

	config.ServerHost = os.Getenv("SERVER_HOST")
	config.ServerPort = os.Getenv("SERVER_PORT")
	config.SecretKey = os.Getenv("SECRET_KEY")
	config.PostgresUrl = os.Getenv("POSTGRES_URL")
	config.ClientDomain = os.Getenv("CLIENT_DOMAIN")

	return *config
}
