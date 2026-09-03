package environment

import "os"

func IsDevelopment() bool {
	return os.Getenv("ENVIRONMENT") == "DEV"
}

func IsMiyoo() bool {
	return os.Getenv("IS_MIYOO") == "1"
}
