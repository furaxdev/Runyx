package main

import (
	"fmt"

	"github.com/furaxdev/runyx/internal/version"
)

func cmdVersion(args []string) error {
	fmt.Println("runyx " + version.String())
	return nil
}
