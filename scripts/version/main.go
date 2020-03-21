package main

import (
	"fmt"

	"github.ibm.com/symposium/marketplace-operator/version"
)

func main() {
	_, _ = fmt.Printf("%s", version.Version)
}
