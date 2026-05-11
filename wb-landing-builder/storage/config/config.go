package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	LogLevel    string
	APISecret   string

	JWTSecret            string
	JWTExpiration        time.Duration
	JWTRefreshExpiration time.Duration

	DBConfig     DatabaseConfig
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	RateLimit    int
}

type DatabaseConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Database       string
	MaxConnections int
	TtlDays        int
}

func Load() *Config {
	if err := godotenv.Load("/app/config/.env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbPort, _ := strconv.Atoi(getEnv("MONGO_PORT", "27017"))
	maxConnections, _ := strconv.Atoi(getEnv("MONGO_MAX_CONNECTIONS", "100"))
	ttlDays, _ := strconv.Atoi(getEnv("MONGO_TTL_DAYS", "30"))
	rateLimit, _ := strconv.Atoi(getEnv("RATE_LIMIT", "100"))

	readTimeout, _ := time.ParseDuration(getEnv("READ_TIMEOUT", "10s"))
	writeTimeout, _ := time.ParseDuration(getEnv("WRITE_TIMEOUT", "10s"))

	jwtExpiration, _ := time.ParseDuration(getEnv("JWT_EXPIRATION", "15m"))
	jwtRefreshExpiration, _ := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRATION", "168h"))

	return &Config{
		Port:                 getEnv("PORT", "8080"),
		Environment:          getEnv("ENVIRONMENT", "production"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		APISecret:            getEnv("API_SECRET", "stub"),
		JWTSecret:            getEnv("JWT_SECRET", "dev-secret"),
		JWTExpiration:        jwtExpiration,
		JWTRefreshExpiration: jwtRefreshExpiration,
		ReadTimeout:          readTimeout,
		WriteTimeout:         writeTimeout,
		RateLimit:            rateLimit,
		DBConfig: DatabaseConfig{
			Host:           getEnv("MONGO_HOST", "mongo"),
			Port:           dbPort,
			User:           getEnv("MONGO_USER", "admin"),
			Password:       getEnv("MONGO_PASSWORD", "admin"),
			Database:       getEnv("MONGO_DATABASE", "storage"),
			MaxConnections: maxConnections,
			TtlDays:        ttlDays,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (cfg *Config) GetMongoURI() string {
	c := cfg.DBConfig
	uri := fmt.Sprintf("mongodb://%s:%d", c.Host, c.Port)

	if c.User != "" && c.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d",
			url.QueryEscape(c.User),
			url.QueryEscape(c.Password),
			c.Host,
			c.Port,
		)
	}
	return uri
}
