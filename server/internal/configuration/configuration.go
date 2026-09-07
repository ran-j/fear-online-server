package configuration

import (
	"fmt"
	"os"
	"strconv"

	"github.com/thoas/go-funk"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/joho/godotenv"
)

type ApplicationEnvEnum string

const (
	Development ApplicationEnvEnum = "DEVELOPMENT"
	Test        ApplicationEnvEnum = "TEST"
	Production  ApplicationEnvEnum = "PRODUCTION"
	Staging     ApplicationEnvEnum = "STAGING"
)

type AppConfig struct {
	Port            string `env:"PORT"`
	ApplicationEnv  string `env:"APPLICATION_ENV"`
	IsTesting       bool   `env:"TESTING"`
	ServiceName     string `env:"SERVICE_NAME"`
	LoginServerIP   string `env:"LOGIN_SERVER_IP"`
	LoginServerPort string `env:"LOGIN_SERVER_PORT"`
	MongoURI        string `env:"MONGODB_URI"`
	MongoDatabase   string `env:"MONGODB_DATABASE"`
	StartingPoint   uint64 `env:"STARTING_POINT"`
	StartingCash    uint64 `env:"STARTING_CASH"`
}

func (appConfig AppConfig) IsDevelopmentEnvironment() bool {
	developmentEnvironments := []ApplicationEnvEnum{Test, Development, Staging}
	return funk.Contains(developmentEnvironments, ApplicationEnvEnum(appConfig.ApplicationEnv))
}

func (appConfig AppConfig) Validate() error {
	return validation.ValidateStruct(
		&appConfig,
		validation.Field(&appConfig.Port, validation.Required),
		validation.Field(&appConfig.ApplicationEnv, validation.Required),
	)
}

func GetEnvString(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return ""
}

func GetEnvStringOr(key, fallback string) string {
	if value := GetEnvString(key); value != "" {
		return value
	}
	return fallback
}

func GetEnvUint64Or(key string, fallback uint64) uint64 {
	if value := GetEnvString(key); value != "" {
		if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func GetEnvBool(key string) bool {
	if value, ok := os.LookupEnv(key); ok {
		if value, err := strconv.ParseBool(value); err == nil {
			return value
		}
	}
	return false
}

func Load() (*AppConfig, error) {
	config := AppConfig{}
	err := godotenv.Load()
	if err != nil {
		fmt.Println("[WARN] .env file not found. Loading from system environment")
	}
	config.Port = GetEnvString("PORT")
	config.ApplicationEnv = GetEnvString("APPLICATION_ENV")
	config.IsTesting = GetEnvBool("TESTING")
	config.ServiceName = GetEnvString("SERVICE_NAME")
	config.LoginServerIP = GetEnvStringOr("LOGIN_SERVER_IP", "127.0.0.1")
	config.LoginServerPort = GetEnvStringOr("LOGIN_SERVER_PORT", "30003")
	config.MongoURI = GetEnvStringOr("MONGODB_URI", "mongodb://localhost:27017")
	config.MongoDatabase = GetEnvStringOr("MONGODB_DATABASE", "fear_online")
	config.StartingPoint = GetEnvUint64Or("STARTING_POINT", 50000)
	config.StartingCash = GetEnvUint64Or("STARTING_CASH", 50000)
	return &config, config.Validate()
}
