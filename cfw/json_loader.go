package cfw

import (
	"embed"

	"grout/internal/jsonutil"
)

//go:embed nextui muos knulli spruce crossmix
var embeddedFiles embed.FS

func mustLoadJSONMap[K comparable, V any](path string) map[K]V {
	return jsonutil.MustLoadJSONMap[K, V](embeddedFiles, path, "cfw")
}
