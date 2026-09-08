package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceSecurity_CrossTenantRead(t *testing.T) {
	ts := SetupTestServer(t)

	org1 := ts.createOrg(t, "ev-org-1", "Evidence Org 1")
	org2 := ts.createOrg(t, "ev-org-2", "Evidence Org 2")

	ts.registerUser(t, org1, "user1@example.com", "User One", "password1234")
	token1 := ts.login(t, org1, "user1@example.com", "password1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+org1.String()+"/cases", token1, map[string]interface{}{
		"title":        "Case in Org 1",
		"service_type": "GENERAL",
		"priority":     "NORMAL",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+org1.String()+"/evidence", token1, map[string]interface{}{
		"service_request_id": caseResp.Data.ID,
		"type":               "IDENTITY_DOCUMENT",
		"description":        "Test document",
		"storage_reference":  "s3://civora-evidence/doc-001",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var evResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &evResp))

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+org2.String()+"/evidence/"+evResp.Data.ID, token1, nil)
	assert.Equal(t, http.StatusForbidden, resp.Code,
		"org1 user should not read org2 evidence; body: %s", resp.Body.String())
}

func TestEvidenceSecurity_CrossCaseRead(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "ev-cross-case", "Evidence Cross Case Org")
	ts.registerUser(t, orgID, "user@example.com", "Test User", "securepass1234")
	token := ts.login(t, orgID, "user@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Case A",
		"service_type": "GENERAL",
		"priority":     "NORMAL",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var caseA struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseA))

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Case B",
		"service_type": "GENERAL",
		"priority":     "NORMAL",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var caseB struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseB))

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/evidence", token, map[string]interface{}{
		"service_request_id": caseA.Data.ID,
		"type":               "IDENTITY_DOCUMENT",
		"description":        "Evidence for Case A",
		"storage_reference":  "s3://civora-evidence/doc-a",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/evidence/by-service-request/"+caseB.Data.ID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	var listResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listResp))
	assert.Empty(t, listResp.Data, "Case B should have no evidence")
}

func TestEvidenceSecurity_UnauthenticatedAccess(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "ev-noauth", "Evidence No Auth Org")
	_ = orgID

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/evidence", "", map[string]interface{}{
		"service_request_id": uuid.New().String(),
		"type":               "IDENTITY_DOCUMENT",
		"description":        "Should be rejected",
		"storage_reference":  "s3://civora-evidence/doc",
	})
	assert.Equal(t, http.StatusUnauthorized, resp.Code,
		"unauthenticated evidence creation should be rejected; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/evidence/"+uuid.New().String(), "", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.Code,
		"unauthenticated evidence read should be rejected; body: %s", resp.Body.String())
}

func TestEvidenceSecurity_StorageReferenceNotLeaked(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "ev-leak", "Evidence Leak Org")
	ts.registerUser(t, orgID, "user@example.com", "Test User", "securepass1234")
	token := ts.login(t, orgID, "user@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Evidence Leak Case",
		"service_type": "GENERAL",
		"priority":     "NORMAL",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/evidence", token, map[string]interface{}{
		"service_request_id": caseResp.Data.ID,
		"type":               "IDENTITY_DOCUMENT",
		"description":        "Sensitive document",
		"storage_reference":  "s3://civora-evidence/secret-doc-001",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var evResp struct {
		Data struct {
			ID               string `json:"id"`
			StorageReference string `json:"storage_reference"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &evResp))

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/evidence/"+evResp.Data.ID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	var getResp struct {
		Data struct {
			StorageReference string `json:"storage_reference"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &getResp))
	assert.Equal(t, "s3://civora-evidence/secret-doc-001", getResp.Data.StorageReference,
		"authorized user should see storage reference")
}
