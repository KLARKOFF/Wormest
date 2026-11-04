// Package config читает настройки сервера из переменных окружения.
package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	StorageDir       string        // куда складывать загруженные файлы
	CookieSecure     bool          // true, если приложение открыто по HTTPS
	RegistrationOpen bool          // можно закрыть регистрацию после создания своих аккаунтов
	SessionTTL       time.Duration // сколько живёт вход на устройстве
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		StorageDir:       env("STORAGE_DIR", "./data/files"),
		CookieSecure:     envBool("COOKIE_SECURE", false),
		RegistrationOpen: envBool("REGISTRATION_OPEN", true),
		SessionTTL:       30 * 24 * time.Hour,
	}
	if c.DatabaseURL == "" {
		return c, errors.New("переменная DATABASE_URL не задана")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
