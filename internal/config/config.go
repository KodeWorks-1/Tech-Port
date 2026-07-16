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
	dbURL := getenv("DATABASE_URL", "")
	if dbURL == "" {
		dbURL = getenv("POSTGRES_URL", "") // Vercel/Neon integration name
	}
	if dbURL == "" {
		dbURL = "postgres://techport:techport@localhost:5543/techport?sslmode=disable"
	}
	return Config{
		Env:         getenv("ENV", "dev"),
		Port:        getenv("PORT", "8080"),
		DatabaseURL: dbURL,
	}
}

// Dev reports whether to use disk templates/statics with live reload.
// Requires the views directory to actually exist on disk, so a binary
// deployed without the repo (Vercel) always uses its embedded assets even
// if ENV is unset.
func (c Config) Dev() bool {
	if c.Env != "dev" {
		return false
	}
	_, err := os.Stat("views")
	return err == nil
}

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
