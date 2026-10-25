package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	DiscordClientID     string
	DiscordClientSecret string
	DiscordBotToken     string
	DiscordRedirectURL  string
	FrontendURL         string
	CookieSecure        bool
	// Clave de 32 bytes con la que se cifran las API keys de los LLM. Vacía: la IA queda desactivada.
	AIEncryptionKey []byte
	// Permite que la URL del LLM apunte a direcciones privadas o locales (solo para desarrollo).
	AIAllowPrivateURLs bool
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:            get("HTTP_ADDR", ":8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordBotToken:     os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordRedirectURL:  get("DISCORD_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		FrontendURL:         get("FRONTEND_URL", "http://localhost:5173"),
		CookieSecure:        os.Getenv("COOKIE_SECURE") == "true",
	}

	if v := os.Getenv("AI_ENCRYPTION_KEY"); v != "" {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(key) != 32 {
			return Config{}, fmt.Errorf("AI_ENCRYPTION_KEY debe ser una clave de 32 bytes en base64 (openssl rand -base64 32)")
		}
		c.AIEncryptionKey = key
	}
	c.AIAllowPrivateURLs = os.Getenv("AI_ALLOW_PRIVATE_URLS") == "true"

	for name, v := range map[string]string{
		"DATABASE_URL":          c.DatabaseURL,
		"DISCORD_CLIENT_ID":     c.DiscordClientID,
		"DISCORD_CLIENT_SECRET": c.DiscordClientSecret,
		"DISCORD_BOT_TOKEN":     c.DiscordBotToken,
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
