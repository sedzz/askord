// Package db embebe las migraciones SQL para aplicarlas al arrancar la API.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
