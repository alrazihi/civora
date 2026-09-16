package identity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	canonicalIdentityJSON = "eyJtb2R1bGVfcGF0aCI6ImdpdGh1Yi5jb20vYWxyYXppaGkvY2l2b3JhIiwicHJvZHVjdF9uYW1lIjoiQ0lWT1JBIiwicHJvamVjdF9pZGVudGlmaWVyIjoiY2l2b3JhIn0="

	publicKeyB64 = "OaRMqXXPDrXmnZ6jNiBlMo7rMfTNikJgcDGYVgZ6+Cs="

	signatureB64 = "jV6PIQqu2Y+kEqnnnOdmVkbzjOZ3D7ou6Lsjq7UE83lPAEV7MyHT/i83VajceFNJkH+FmyOkYIueFIOreSSLCw=="
)

type canonicalIdentityStruct struct {
	ModulePath        string `json:"module_path"`
	ProductName       string `json:"product_name"`
	ProjectIdentifier string `json:"project_identifier"`
}

var canonicalIdentityBytes = func() []byte {
	id := canonicalIdentityStruct{
		ModulePath:        ModulePath,
		ProductName:       ProductName,
		ProjectIdentifier: ProjectIdentifier,
	}
	b, _ := json.Marshal(id)
	return b
}()

type IntegrityError struct {
	Expected string
	Detected string
}

func (e *IntegrityError) Error() string {
	return fmt.Sprintf(
		"CIVORA INTEGRITY CHECK FAILED\n\n"+
			"The official CIVORA project identity has been modified.\n\n"+
			"Expected identity: %s\n"+
			"Detected identity: %s\n\n"+
			"Restore the official CIVORA identity or use an authorized derivative/deployment process.",
		e.Expected, e.Detected,
	)
}

func VerifyIntegrity() *IntegrityError {
	embeddedIdentity, err := base64.StdEncoding.DecodeString(canonicalIdentityJSON)
	if err != nil {
		return &IntegrityError{
			Expected: ProductName,
			Detected: fmt.Sprintf("failed to decode embedded identity: %v", err),
		}
	}

	var embedded struct {
		ModulePath        string `json:"module_path"`
		ProductName       string `json:"product_name"`
		ProjectIdentifier string `json:"project_identifier"`
	}
	if err := json.Unmarshal(embeddedIdentity, &embedded); err != nil {
		return &IntegrityError{
			Expected: ProductName,
			Detected: fmt.Sprintf("failed to parse embedded identity: %v", err),
		}
	}

	if embedded.ProductName != ProductName {
		return &IntegrityError{Expected: ProductName, Detected: embedded.ProductName}
	}
	if embedded.ProjectIdentifier != ProjectIdentifier {
		return &IntegrityError{Expected: ProjectIdentifier, Detected: embedded.ProjectIdentifier}
	}
	if embedded.ModulePath != ModulePath {
		return &IntegrityError{Expected: ModulePath, Detected: embedded.ModulePath}
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return &IntegrityError{
			Expected: ProductName,
			Detected: fmt.Sprintf("failed to decode public key: %v", err),
		}
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return &IntegrityError{
			Expected: ProductName,
			Detected: fmt.Sprintf("failed to decode signature: %v", err),
		}
	}

	if !ed25519.Verify(pubKeyBytes, embeddedIdentity, sigBytes) {
		return &IntegrityError{
			Expected: ProductName,
			Detected: "signature verification failed",
		}
	}

	return nil
}

func MustVerifyIntegrity() {
	if err := VerifyIntegrity(); err != nil {
		panic(err.Error())
	}
}

func Fingerprint() string {
	h := sha256.Sum256(canonicalIdentityBytes)
	return fmt.Sprintf("%x", h[:])
}

var (
	ErrIdentityMismatch = errors.New("CIVORA identity mismatch")
)
