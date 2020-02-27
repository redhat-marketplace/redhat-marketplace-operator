package utils

import "os"

// Getenv will return the value for the passed environment variable or if doesn't exist the fallback
func Getenv(key, fallback string) string {

	println(key)
	image, found := os.LookupEnv(key)
	if !found {
		return fallback
	}
	return image
}
