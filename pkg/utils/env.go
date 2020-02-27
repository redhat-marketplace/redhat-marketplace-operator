package utils

import "os"

// Getenv will return the value for the passed environment variable or if doesn't exist the fallback
func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}
