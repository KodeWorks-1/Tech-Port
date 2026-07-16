package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Env         string
	Port        string
	DatabaseURL string
}

// Load reads .env (if present) into the process environment, then builds the
// config from environment variables with dev-friendly defaults.
func Load() Config {
	loadDotEnv(".env")
	return Config{
		Env:         getenv("ENV", "dev"),
		Port:        getenv("PORT", "8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://techport:techport@localhost:5543/techport?sslmode=disable"),
	}
}

func (c Config) Dev() bool { return c.Env == "dev" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
