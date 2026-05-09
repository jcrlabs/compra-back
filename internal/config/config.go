package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const minJWTSecretLength = 32

type Config struct {
	Port           string
	Env            string
	AllowedOrigins []string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret         string
	JWTAccessTTLHours int

	ScraperCron         string
	ScraperUserAgent    string
	MercadonaPostalCode string

	AdminSecret string
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

func Load() (*Config, error) {
	_ = godotenv.Load()

	jwtSecret := getEnv("JWT_SECRET", "")
	if len(jwtSecret) < minJWTSecretLength {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d chars (got %d)", minJWTSecretLength, len(jwtSecret))
	}

	accessTTL, _ := strconv.Atoi(getEnv("JWT_ACCESS_TTL_HOURS", "24"))
	if accessTTL <= 0 {
		accessTTL = 24
	}

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
	var origins []string
	for _, o := range strings.Split(originsRaw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}

	return &Config{
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("ENV", "development"),
		AllowedOrigins: origins,

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "compra"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "compra"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),

		JWTSecret:         jwtSecret,
		JWTAccessTTLHours: accessTTL,

		ScraperCron:         getEnv("SCRAPER_CRON", "0 3 * * *"),
		ScraperUserAgent:    getEnv("SCRAPER_USER_AGENT", "Mozilla/5.0 (compatible; jcrlabs-bot/1.0)"),
		MercadonaPostalCode: getEnv("MERCADONA_POSTAL_CODE", "28001"),

		AdminSecret: getEnv("ADMIN_SECRET", ""),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
