package utils

import "os"

func Getenv(key, fallback string) string {

	image, found := os.LookupEnv(key)
	if !found {
		return fallback
	}
	return image
}
