package config

import (
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURL  string
	FrontendURL         string
	CookieSecure        bool
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:            get("HTTP_ADDR", ":8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  get("DISCORD_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		FrontendURL:         get("FRONTEND_URL", "http://localhost:5173"),
		CookieSecure:        os.Getenv("COOKIE_SECURE") == "true",
	}

	for name, v := range map[string]string{
		"DATABASE_URL":          c.DatabaseURL,
		"DISCORD_CLIENT_ID":     c.DiscordClientID,
		"DISCORD_CLIENT_SECRET": c.DiscordClientSecret,
	} {
		if v == "" {
			return Config{}, fmt.Errorf("falta la variable de entorno %s", name)
		}
	}
	return c, nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
