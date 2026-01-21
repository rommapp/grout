package environment

import "os"

func IsDevelopment() bool {
	return os.Getenv("ENVIRONMENT") == "DEV"
}
