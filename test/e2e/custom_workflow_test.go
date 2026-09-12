package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomWorkflowEndToEnd(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "custom-wf-"+uuid.New().String()[:8], "Custom Workflow Org")
	ts.registerUser(t, orgID, "admin@example.com", "Test Admin", "securepass1234")
	token := ts.login(t, orgID, "admin@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Custom", "Person", "en")

	customSupportDefID := createCustomWorkflowDefinition(t, ts, orgID, token, "custom_support",
		"Custom Support Workflow",
		[]interface{}{
			map[string]interface{}{"key": "SNEW", "name": "New Support Ticket", "terminal": false, "display_order": 0},
			map[string]interface{}{"key": "SIN_PROGRESS", "name": "In Progress", "terminal": false, "display_order": 1},
			map[string]interface{}{"key": "RESOLVED", "name": "Resolved", "terminal": false, "display_order": 2},
			map[string]interface{}{"key": "CLOSED", "name": "Closed", "terminal": true, "display_order": 3},
		},
		[]interface{}{
			map[string]interface{}{"key": "start", "name": "Start", "from_state": "SNEW", "to_state": "SIN_PROGRESS", "active": true},
			map[string]interface{}{"key": "resolve", "name": "Resolve", "from_state": "SIN_PROGRESS", "to_state": "RESOLVED", "active": true},
			map[string]interface{}{"key": "close", "name": "Close", "from_state": "RESOLVED", "to_state": "CLOSED", "active": true},
		},
		"SNEW",
	)

	assert.Equal(t, "ACTIVE", getWorkflowDefinitionStatus(t, ts, orgID, token, customSupportDefID))

	caseID := createCaseWithWorkflow(t, ts, orgID, token, "Custom Support Case", "Support ticket workflow test", "GENERAL", "NORMAL", personID, customSupportDefID)

	verifyWorkflowState(t, ts, orgID, token, caseID, "SNEW", customSupportDefID)

	transitionWorkflow(t, ts, orgID, token, caseID, "start")
	verifyWorkflowState(t, ts, orgID, token, caseID, "SIN_PROGRESS", customSupportDefID)

	transitionWorkflow(t, ts, orgID, token, caseID, "resolve")
	verifyWorkflowState(t, ts, orgID, token, caseID, "RESOLVED", customSupportDefID)

	transitionWorkflow(t, ts, orgID, token, caseID, "close")
	verifyWorkflowState(t, ts, orgID, token, caseID, "CLOSED", customSupportDefID)

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/start", token, nil)
	require.Equal(t, http.StatusConflict, resp.Code, "terminal state should block further transitions")

	histories := getWorkflowHistory(t, ts, orgID, token, caseID)
	require.Len(t, histories, 3, "should have 3 transition history entries")
	assert.Equal(t, "SNEW", histories[0].FromState)
	assert.Equal(t, "SIN_PROGRESS", histories[0].ToState)
	assert.Equal(t, "start", histories[0].TransitionKey)
	assert.Equal(t, "SIN_PROGRESS", histories[1].FromState)
	assert.Equal(t, "RESOLVED", histories[1].ToState)
	assert.Equal(t, "resolve", histories[1].TransitionKey)
	assert.Equal(t, "RESOLVED", histories[2].FromState)
	assert.Equal(t, "CLOSED", histories[2].ToState)
	assert.Equal(t, "close", histories[2].TransitionKey)

	cases := getCaseDetail(t, ts, orgID, token, caseID)
	assert.Equal(t, "CLOSED", cases.Status)
	require.NotNil(t, cases.WorkflowState)
	assert.Equal(t, "CLOSED", *cases.WorkflowState)
	assert.NotNil(t, cases.ClosedAt, "case should have closed_at set")

	auditEvents := listAuditEvents(t, ts, orgID, token)
	var wfTransitions int
	for _, ev := range auditEvents {
		if ev.Action == "workflow.transition" {
			wfTransitions++
		}
	}
	assert.GreaterOrEqual(t, wfTransitions, 3, "should have at least 3 workflow transition audit events")
}

func TestSecondCustomWorkflowEndToEnd(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-edu-"+uuid.New().String()[:8], "Education Workflow Org")
	ts.registerUser(t, orgID, "admin@example.com", "Test Admin", "securepass1234")
	token := ts.login(t, orgID, "admin@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Education", "Student", "en")

	educationDefID := createCustomWorkflowDefinition(t, ts, orgID, token, "edu_custom_workflow",
		"Education Assistance Workflow",
		[]interface{}{
			map[string]interface{}{"key": "EDU_NEW", "name": "New Request", "terminal": false, "display_order": 0},
			map[string]interface{}{"key": "EDU_REVIEW", "name": "Under Review", "terminal": false, "display_order": 1},
			map[string]interface{}{"key": "EDU_APPROVED", "name": "Approved", "terminal": false, "display_order": 2},
			map[string]interface{}{"key": "EDU_REJECTED", "name": "Rejected", "terminal": false, "display_order": 3},
			map[string]interface{}{"key": "CLOSED", "name": "Closed", "terminal": true, "display_order": 4},
		},
		[]interface{}{
			map[string]interface{}{"key": "review", "name": "Review", "from_state": "EDU_NEW", "to_state": "EDU_REVIEW", "active": true},
			map[string]interface{}{"key": "approve", "name": "Approve", "from_state": "EDU_REVIEW", "to_state": "EDU_APPROVED", "active": true},
			map[string]interface{}{"key": "reject", "name": "Reject", "from_state": "EDU_REVIEW", "to_state": "EDU_REJECTED", "active": true},
			map[string]interface{}{"key": "close_edu", "name": "Close", "from_state": "EDU_APPROVED", "to_state": "CLOSED", "active": true},
			map[string]interface{}{"key": "close_rejected", "name": "Close Rejected", "from_state": "EDU_REJECTED", "to_state": "CLOSED", "active": true},
		},
		"EDU_NEW",
	)

	assert.Equal(t, "ACTIVE", getWorkflowDefinitionStatus(t, ts, orgID, token, educationDefID))

	eduCaseID := createCaseWithWorkflow(t, ts, orgID, token, "Education Assistance Case", "Education workflow test", "GENERAL", "NORMAL", personID, educationDefID)

	verifyWorkflowState(t, ts, orgID, token, eduCaseID, "EDU_NEW", educationDefID)

	transitionWorkflow(t, ts, orgID, token, eduCaseID, "review")
	verifyWorkflowState(t, ts, orgID, token, eduCaseID, "EDU_REVIEW", educationDefID)

	transitionWorkflow(t, ts, orgID, token, eduCaseID, "approve")
	verifyWorkflowState(t, ts, orgID, token, eduCaseID, "EDU_APPROVED", educationDefID)

	transitionWorkflow(t, ts, orgID, token, eduCaseID, "close_edu")
	verifyWorkflowState(t, ts, orgID, token, eduCaseID, "CLOSED", educationDefID)

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+eduCaseID+"/workflow/transitions/review", token, nil)
	require.Equal(t, http.StatusConflict, resp.Code, "terminal state should block further transitions")

	eduHistories := getWorkflowHistory(t, ts, orgID, token, eduCaseID)
	require.Len(t, eduHistories, 3, "should have 3 transition history entries for education workflow")
	assert.Equal(t, "review", eduHistories[0].TransitionKey)
	assert.Equal(t, "approve", eduHistories[1].TransitionKey)
	assert.Equal(t, "close_edu", eduHistories[2].TransitionKey)

	eduCases := getCaseDetail(t, ts, orgID, token, eduCaseID)
	assert.Equal(t, "CLOSED", eduCases.Status)
	require.NotNil(t, eduCases.WorkflowState)
	assert.Equal(t, "CLOSED", *eduCases.WorkflowState)
	assert.NotNil(t, eduCases.ClosedAt, "case should have closed_at set")
}

func TestCustomWorkflowCaseViaCaseStatusEndpoint(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-status-"+uuid.New().String()[:8], "Status Workflow Org")
	ts.registerUser(t, orgID, "admin@example.com", "Test Admin", "securepass1234")
	token := ts.login(t, orgID, "admin@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Status", "Person", "en")

	defID := createCustomWorkflowDefinition(t, ts, orgID, token, "status_workflow",
		"Status Workflow",
		[]interface{}{
			map[string]interface{}{"key": "STATUS_NEW", "name": "New", "terminal": false, "display_order": 0},
			map[string]interface{}{"key": "CLOSED", "name": "Done", "terminal": true, "display_order": 1},
		},
		[]interface{}{
			map[string]interface{}{"key": "complete", "name": "Complete", "from_state": "STATUS_NEW", "to_state": "CLOSED", "active": true},
		},
		"STATUS_NEW",
	)

	caseID := createCaseWithWorkflow(t, ts, orgID, token, "Status Case", "Status sync test", "GENERAL", "NORMAL", personID, defID)

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "CLOSED",
	})
	require.Equal(t, http.StatusOK, resp.Code, "case status transition should work: %s", resp.Body.String())

	cases := getCaseDetail(t, ts, orgID, token, caseID)
	assert.Equal(t, "CLOSED", cases.Status)
	require.NotNil(t, cases.WorkflowState)
	assert.Equal(t, "CLOSED", *cases.WorkflowState)
	assert.NotNil(t, cases.ClosedAt, "case should be closed")
}

func TestCustomWorkflowSelectable(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-select-"+uuid.New().String()[:8], "Selectable Workflow Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/workflows", token, map[string]interface{}{
		"key":           "selectable_workflow",
		"name":          "Selectable Workflow",
		"version":       1,
		"initial_state": "START",
		"states": []map[string]interface{}{
			{"key": "START", "name": "Start", "terminal": false, "display_order": 0},
			{"key": "END", "name": "End", "terminal": true, "display_order": 1},
		},
		"transitions": []map[string]interface{}{
			{"key": "finish", "name": "Finish", "from_state": "START", "to_state": "END", "active": true},
		},
		"metadata": map[string]interface{}{},
	})
	require.Equal(t, http.StatusCreated, resp.Code)

	var defResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &defResp))

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/workflows/"+defResp.Data.ID+"/activate", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/workflows/selectable", token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	var listResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &listResp))

	var found bool
	for _, wf := range listResp.Data {
		if wf["key"] == "selectable_workflow" {
			found = true
			assert.Equal(t, "ACTIVE", wf["status"])
			assert.Equal(t, "START", wf["initial_state"])
		}
	}
	assert.True(t, found, "selectable_workflow should be in active workflow selection list")
}

func createCustomWorkflowDefinition(t *testing.T, ts *TestServer, orgID uuid.UUID, token, key, name string, states, transitions []interface{}, initialState string) string {
	t.Helper()
	body := map[string]interface{}{
		"key":           key,
		"name":          name,
		"version":       1,
		"initial_state": initialState,
		"states":        states,
		"transitions":   transitions,
		"metadata":      map[string]interface{}{},
	}
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/workflows", token, body)
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var result struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, "DRAFT", result.Data.Status)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/workflows/"+result.Data.ID+"/activate", token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	return result.Data.ID
}

func getWorkflowDefinitionStatus(t *testing.T, ts *TestServer, orgID uuid.UUID, token, defID string) string {
	t.Helper()
	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/workflows/"+defID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)
	var result struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.Status
}

func createCaseWithWorkflow(t *testing.T, ts *TestServer, orgID uuid.UUID, token, title, description, serviceType, priority, personID, workflowID string) string {
	t.Helper()
	body := map[string]interface{}{
		"title":        title,
		"description":  description,
		"service_type": serviceType,
		"priority":     priority,
		"workflow_id":  workflowID,
	}
	if personID != "" {
		body["person_id"] = personID
	}
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, body)
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.ID
}

func verifyWorkflowState(t *testing.T, ts *TestServer, orgID uuid.UUID, token, caseID, expectedState, expectedDefID string) {
	t.Helper()
	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow", token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			Instance struct {
				CurrentState         string `json:"current_state"`
				WorkflowDefinitionID string `json:"workflow_definition_id"`
			} `json:"instance"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	assert.Equal(t, expectedState, result.Data.Instance.CurrentState, "workflow instance should be in expected state")
	assert.Equal(t, expectedDefID, result.Data.Instance.WorkflowDefinitionID, "workflow instance should reference the correct definition")
}

func transitionWorkflow(t *testing.T, ts *TestServer, orgID uuid.UUID, token, caseID, transitionKey string) {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/"+transitionKey, token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "transition %s should succeed: %s", transitionKey, resp.Body.String())
}

type workflowTransitionHistoryEntry struct {
	FromState     string `json:"from_state"`
	ToState       string `json:"to_state"`
	TransitionKey string `json:"transition_key"`
}

func getWorkflowHistory(t *testing.T, ts *TestServer, orgID uuid.UUID, token, caseID string) []workflowTransitionHistoryEntry {
	t.Helper()
	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/history", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)
	var result struct {
		Data []workflowTransitionHistoryEntry `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data
}

type auditEventSummary struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Outcome  string `json:"outcome"`
}

func listAuditEvents(t *testing.T, ts *TestServer, orgID uuid.UUID, token string) []auditEventSummary {
	t.Helper()
	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)
	var result struct {
		Data []auditEventSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data
}

type caseDetail struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	WorkflowState *string `json:"workflow_state"`
	ClosedAt      *string `json:"closed_at"`
}

func getCaseDetail(t *testing.T, ts *TestServer, orgID uuid.UUID, token, caseID string) *caseDetail {
	t.Helper()
	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)
	var result struct {
		Data *caseDetail `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data
}
