package main

import (
	"fmt"
	"os"

	"github.com/alrazihi/civora/internal/identity"
)

func main() {
	if err := identity.VerifyIntegrity(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Printf("CIVORA identity verified: %s (fingerprint: %s)\n",
		identity.ProjectIdentifier, identity.Fingerprint())
}
