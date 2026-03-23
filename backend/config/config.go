package config

import (
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
	switch err {
	case nil:
		log.Println("Config loaded successfully from .env file")
	case os.ErrNotExist:
		log.Println("No .env file found, using environment variables")
	default:
		log.Fatalf("Error loading .env file: %s", err)
	}

	config.ServerHost = os.Getenv("SERVER_HOST")
	config.ServerPort = os.Getenv("SERVER_PORT")
	config.SecretKey = os.Getenv("SECRET_KEY")
	config.PostgresUrl = os.Getenv("POSTGRES_URL")
	config.ClientDomain = os.Getenv("CLIENT_DOMAIN")

	return *config
}
