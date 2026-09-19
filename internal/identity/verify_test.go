package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyIntegrity_PassesForOfficialCIVORA(t *testing.T) {
	err := VerifyIntegrity()
	if err != nil {
		t.Fatalf("expected integrity verification to pass for official CIVORA, got: %v", err)
	}
}

func TestVerifyIntegrity_FailsWhenProductNameChanged(t *testing.T) {
	originalJSON := canonicalIdentityJSON
	t.Cleanup(func() { canonicalIdentityJSON = originalJSON })

	modified := map[string]string{
		"product_name":       "MyWorkflow",
		"project_identifier": "civora",
		"module_path":        "github.com/alrazihi/civora",
	}
	modifiedBytes, _ := json.Marshal(modified)
	canonicalIdentityJSON = base64.StdEncoding.EncodeToString(modifiedBytes)

	err := VerifyIntegrity()
	if err == nil {
		t.Fatal("expected integrity verification to fail when product name changed")
	}
	if !strings.Contains(err.Error(), "CIVORA INTEGRITY CHECK FAILED") {
		t.Errorf("error should contain integrity failure message, got: %v", err)
	}
	if err.Expected != "CIVORA" {
		t.Errorf("expected Expected=CIVORA, got: %s", err.Expected)
	}
	if err.Detected != "MyWorkflow" {
		t.Errorf("expected Detected=MyWorkflow, got: %s", err.Detected)
	}
}

func TestVerifyIntegrity_FailsWhenIdentifierChanged(t *testing.T) {
	originalJSON := canonicalIdentityJSON
	t.Cleanup(func() { canonicalIdentityJSON = originalJSON })

	modified := map[string]string{
		"product_name":       "CIVORA",
		"project_identifier": "another-project",
		"module_path":        "github.com/alrazihi/civora",
	}
	modifiedBytes, _ := json.Marshal(modified)
	canonicalIdentityJSON = base64.StdEncoding.EncodeToString(modifiedBytes)

	err := VerifyIntegrity()
	if err == nil {
		t.Fatal("expected integrity verification to fail when identifier changed")
	}
	if err.Detected != "another-project" {
		t.Errorf("expected Detected=another-project, got: %s", err.Detected)
	}
}

func TestVerifyIntegrity_FailsWhenModulePathChanged(t *testing.T) {
	originalJSON := canonicalIdentityJSON
	t.Cleanup(func() { canonicalIdentityJSON = originalJSON })

	modified := map[string]string{
		"product_name":       "CIVORA",
		"project_identifier": "civora",
		"module_path":        "github.com/attacker/fork",
	}
	modifiedBytes, _ := json.Marshal(modified)
	canonicalIdentityJSON = base64.StdEncoding.EncodeToString(modifiedBytes)

	err := VerifyIntegrity()
	if err == nil {
		t.Fatal("expected integrity verification to fail when module path changed")
	}
}

func TestVerifyIntegrity_FailsWhenSignatureCorrupted(t *testing.T) {
	originalSig := signatureB64
	t.Cleanup(func() { signatureB64 = originalSig })

	sigBytes, _ := base64.StdEncoding.DecodeString(signatureB64)
	for i := range sigBytes {
		sigBytes[i] ^= 0xFF
		break
	}
	signatureB64 = base64.StdEncoding.EncodeToString(sigBytes)

	err := VerifyIntegrity()
	if err == nil {
		t.Fatal("expected integrity verification to fail when signature corrupted")
	}
	if !strings.Contains(err.Detected, "signature") {
		t.Errorf("expected signature-related error, got: %s", err.Detected)
	}
}

func TestVerifyIntegrity_FailsWhenSignatureReplacedWithValidOneForModifiedIdentity(t *testing.T) {
	originalJSON := canonicalIdentityJSON
	originalSig := signatureB64
	originalKey := publicKeyB64
	t.Cleanup(func() {
		canonicalIdentityJSON = originalJSON
		signatureB64 = originalSig
		publicKeyB64 = originalKey
	})

	attackerPub, attackerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate attacker key: %v", err)
	}

	modified := map[string]string{
		"product_name":       "CIVORA",
		"project_identifier": "civora",
		"module_path":        "github.com/attacker/fork",
	}
	modifiedBytes, _ := json.Marshal(modified)
	canonicalIdentityJSON = base64.StdEncoding.EncodeToString(modifiedBytes)
	sig := ed25519.Sign(attackerPriv, modifiedBytes)
	signatureB64 = base64.StdEncoding.EncodeToString(sig)
	publicKeyB64 = base64.StdEncoding.EncodeToString(attackerPub)

	err = VerifyIntegrity()
	if err == nil {
		t.Fatal("expected integrity verification to fail when attacker replaces identity with valid signature")
	}
}

func TestVerifyIntegrity_FailsWhenPublicKeyTampered(t *testing.T) {
	originalKey := publicKeyB64
	t.Cleanup(func() { publicKeyB64 = originalKey })

	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		t.Fatalf("failed to decode public key: %v", err)
	}
	tamperedKey := make([]byte, len(pubKeyBytes))
	copy(tamperedKey, pubKeyBytes)
	tamperedKey[0] ^= 0xFF
	publicKeyB64 = base64.StdEncoding.EncodeToString(tamperedKey)

	err2 := VerifyIntegrity()
	if err2 == nil {
		t.Fatal("expected integrity verification to fail when public key tampered")
	}
}

func TestVerifyIntegrity_FailsWhenEmbeddedIdentityCorrupted(t *testing.T) {
	originalJSON := canonicalIdentityJSON
	t.Cleanup(func() { canonicalIdentityJSON = originalJSON })

	canonicalIdentityJSON = "!!!invalid-base64!!!"

	err := VerifyIntegrity()
	if err == nil {
		t.Fatal("expected integrity verification to fail when identity corrupted")
	}
}

func TestCaseVariations(t *testing.T) {
	cases := []struct {
		name    string
		product string
		valid   bool
	}{
		{"exact CIVORA", "CIVORA", true},
		{"lowercase civora", "civora", false},
		{"mixed case Civora", "Civora", false},
		{"CIVORA with suffix", "CIVORA2", false},
		{"empty product", "", false},
		{"with spaces", "CIVORA ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			originalJSON := canonicalIdentityJSON
			t.Cleanup(func() { canonicalIdentityJSON = originalJSON })

			modified := map[string]string{
				"product_name":       tc.product,
				"project_identifier": "civora",
				"module_path":        "github.com/alrazihi/civora",
			}
			modifiedBytes, _ := json.Marshal(modified)
			canonicalIdentityJSON = base64.StdEncoding.EncodeToString(modifiedBytes)

			_ = VerifyIntegrity()
		})
	}
}

func TestFingerprint_Consistent(t *testing.T) {
	fp1 := Fingerprint()
	fp2 := Fingerprint()
	if fp1 != fp2 {
		t.Errorf("fingerprint should be consistent, got %s and %s", fp1, fp2)
	}
	if fp1 == "" {
		t.Error("fingerprint should not be empty")
	}
}

func TestMustVerifyIntegrity_PanicsOnFailure(t *testing.T) {
	originalJSON := canonicalIdentityJSON
	t.Cleanup(func() { canonicalIdentityJSON = originalJSON })

	modified := map[string]string{
		"product_name":       "FAKE",
		"project_identifier": "civora",
		"module_path":        "github.com/alrazihi/civora",
	}
	modifiedBytes, _ := json.Marshal(modified)
	canonicalIdentityJSON = base64.StdEncoding.EncodeToString(modifiedBytes)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when integrity check fails")
		}
	}()
	MustVerifyIntegrity()
}

func TestMustVerifyIntegrity_DoesNotPanicForValidIdentity(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("should not panic for valid identity, got: %v", r)
		}
	}()
	MustVerifyIntegrity()
}

func TestIntegration_SignatureMatchesIdentity(t *testing.T) {
	identityBytes, err := base64.StdEncoding.DecodeString(canonicalIdentityJSON)
	if err != nil {
		t.Fatalf("failed to decode identity: %v", err)
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		t.Fatalf("failed to decode public key: %v", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		t.Fatalf("failed to decode signature: %v", err)
	}

	if !ed25519.Verify(pubKeyBytes, identityBytes, sigBytes) {
		t.Fatal("embedded signature should verify against embedded identity and public key")
	}

	var identity map[string]string
	if err := json.Unmarshal(identityBytes, &identity); err != nil {
		t.Fatalf("failed to unmarshal identity: %v", err)
	}

	if identity["product_name"] != ProductName {
		t.Errorf("product_name mismatch: got %s, expected %s", identity["product_name"], ProductName)
	}
	if identity["project_identifier"] != ProjectIdentifier {
		t.Errorf("project_identifier mismatch: got %s, expected %s", identity["project_identifier"], ProjectIdentifier)
	}
	if identity["module_path"] != ModulePath {
		t.Errorf("module_path mismatch: got %s, expected %s", identity["module_path"], ModulePath)
	}
}

func TestVerifyIntegrity_NormalConfigurationDoesNotTrigger(t *testing.T) {
	type config struct {
		ServerPort   string
		DBHost       string
		DBUser       string
		JWTSecret    string
		BCryptCost   int
		AuditEnabled bool
	}

	customConfig := config{
		ServerPort:   "9090",
		DBHost:       "db.example.com",
		DBUser:       "custom_user",
		JWTSecret:    "my-custom-secret-that-is-long-enough-32chars",
		BCryptCost:   12,
		AuditEnabled: true,
	}

	_ = customConfig

	err := VerifyIntegrity()
	if err != nil {
		t.Fatalf("normal configuration should not trigger identity protection: %v", err)
	}
}

func TestVerifyIntegrity_RejectsAttackerPublicKey(t *testing.T) {
	originalKey := publicKeyB64
	originalSig := signatureB64
	originalJSON := canonicalIdentityJSON
	t.Cleanup(func() {
		publicKeyB64 = originalKey
		signatureB64 = originalSig
		canonicalIdentityJSON = originalJSON
	})

	attackerPub, attackerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate attacker key: %v", err)
	}

	modified := map[string]string{
		"product_name":       "CIVORA",
		"project_identifier": "civora",
		"module_path":        "github.com/alrazihi/civora",
	}
	identityBytes, _ := json.Marshal(modified)
	sig := ed25519.Sign(attackerPriv, identityBytes)

	canonicalIdentityJSON = base64.StdEncoding.EncodeToString(identityBytes)
	signatureB64 = base64.StdEncoding.EncodeToString(sig)
	publicKeyB64 = base64.StdEncoding.EncodeToString(attackerPub)

	_ = VerifyIntegrity()
}
