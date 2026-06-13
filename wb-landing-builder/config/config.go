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

	JWTSecret              string
	JWTExpiration          time.Duration
	RefreshTokenExpiration time.Duration

	DBConfig     DatabaseConfig
	S3           S3Config
	Publishing   PublishingConfig
	RabbitMQ     RabbitMQConfig
	CDN           CDNConfig
	PublicBaseURL string
	StaticFilesDir string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	RateLimit    int
}

type S3Config struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	Bucket       string
	UsePathStyle bool
}

type PublishingConfig struct {
	CLIPath string
}

type RabbitMQConfig struct {
	URL   string
	Queue string
}

type CDNConfig struct {
	CachePath string
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
	refreshTokenExpiration, _ := time.ParseDuration(getEnv("REFRESH_TOKEN_EXPIRATION", "168h"))

	s3UsePathStyle, _ := strconv.ParseBool(getEnv("S3_USE_PATH_STYLE", "true"))

	return &Config{
		Port:                   getEnv("PORT", "8080"),
		Environment:            getEnv("ENVIRONMENT", "production"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		APISecret:              getEnv("API_SECRET", "stub"),
		JWTSecret:              getEnv("JWT_SECRET", "dev-secret"),
		JWTExpiration:          jwtExpiration,
		RefreshTokenExpiration: refreshTokenExpiration,
		ReadTimeout:            readTimeout,
		WriteTimeout:           writeTimeout,
		RateLimit:              rateLimit,
		DBConfig: DatabaseConfig{
			Host:           getEnv("MONGO_HOST", "mongo"),
			Port:           dbPort,
			User:           getEnv("MONGO_USER", "admin"),
			Password:       getEnv("MONGO_PASSWORD", "admin"),
			Database:       getEnv("MONGO_DATABASE", "storage"),
			MaxConnections: maxConnections,
			TtlDays:        ttlDays,
		},
		S3: S3Config{
			Endpoint:     getEnv("S3_ENDPOINT", "http://minio:9000"),
			Region:       getEnv("S3_REGION", "us-east-1"),
			AccessKey:    getEnv("S3_ACCESS_KEY", "minioadmin"),
			SecretKey:    getEnv("S3_SECRET_KEY", "minioadmin"),
			Bucket:       getEnv("S3_BUCKET", "publications"),
			UsePathStyle: s3UsePathStyle,
		},
		Publishing: PublishingConfig{
			CLIPath: getEnv("PUBLISHING_CLI_PATH", "/app/cli/generate.py"),
		},
		RabbitMQ: RabbitMQConfig{
			URL:   getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
			Queue: getEnv("RABBITMQ_PUBLISH_QUEUE", "publish.requests"),
		},
		CDN: CDNConfig{
			CachePath: getEnv("CDN_CACHE_PATH", "/var/cache/nginx/publications"),
		},
		PublicBaseURL:  getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		StaticFilesDir: getEnv("STATIC_FILES_DIR", ""),
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
