package application

import (
	"context"
	"database/sql"
	"testing"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/forms/domain"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	assignmentdomain "github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWorkflowDefRepo struct {
	defs map[uuid.UUID]*workflowdomain.WorkflowDefinition
}

func newMockWorkflowDefRepo() *mockWorkflowDefRepo {
	return &mockWorkflowDefRepo{defs: make(map[uuid.UUID]*workflowdomain.WorkflowDefinition)}
}

func (m *mockWorkflowDefRepo) DB() *sql.DB { return nil }
func (m *mockWorkflowDefRepo) Save(ctx context.Context, def *workflowdomain.WorkflowDefinition) error {
	m.defs[def.ID] = def
	return nil
}
func (m *mockWorkflowDefRepo) SaveTx(ctx context.Context, tx *sql.Tx, def *workflowdomain.WorkflowDefinition) error {
	return m.Save(ctx, def)
}
func (m *mockWorkflowDefRepo) Update(ctx context.Context, def *workflowdomain.WorkflowDefinition) error {
	m.defs[def.ID] = def
	return nil
}
func (m *mockWorkflowDefRepo) UpdateTx(ctx context.Context, tx *sql.Tx, def *workflowdomain.WorkflowDefinition) error {
	return m.Update(ctx, def)
}
func (m *mockWorkflowDefRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	delete(m.defs, id)
	return nil
}
func (m *mockWorkflowDefRepo) DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error {
	return m.Delete(ctx, tenantID, id)
}
func (m *mockWorkflowDefRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error) {
	def, ok := m.defs[id]
	if !ok {
		return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
	}
	return def, nil
}
func (m *mockWorkflowDefRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error) {
	return m.FindByID(ctx, tenantID, id)
}
func (m *mockWorkflowDefRepo) FindByKeyAndVersion(ctx context.Context, tenantID uuid.UUID, key string, version int) (*workflowdomain.WorkflowDefinition, error) {
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: uuid.Nil}
}
func (m *mockWorkflowDefRepo) FindByKey(ctx context.Context, tenantID uuid.UUID, key string) (*workflowdomain.WorkflowDefinition, error) {
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: uuid.Nil}
}
func (m *mockWorkflowDefRepo) FindByKeyTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, key string) (*workflowdomain.WorkflowDefinition, error) {
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: uuid.Nil}
}
func (m *mockWorkflowDefRepo) FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*workflowdomain.WorkflowDefinition, error) {
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: uuid.Nil}
}
func (m *mockWorkflowDefRepo) FindLatestActiveByKeyTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, key string) (*workflowdomain.WorkflowDefinition, error) {
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: uuid.Nil}
}
func (m *mockWorkflowDefRepo) ListByOrganization(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*workflowdomain.WorkflowDefinition, int, error) {
	return nil, 0, nil
}
func (m *mockWorkflowDefRepo) ListActiveByOrganization(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*workflowdomain.WorkflowDefinition, int, error) {
	return nil, 0, nil
}
func (m *mockWorkflowDefRepo) GetActiveByID(ctx context.Context, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error) {
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
}
func (m *mockWorkflowDefRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status workflowdomain.WorkflowDefinitionStatus, version int) error {
	return nil
}
func (m *mockWorkflowDefRepo) UpdateStatusTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, status workflowdomain.WorkflowDefinitionStatus, version int) error {
	return nil
}

type mockFormRepo struct {
	forms map[uuid.UUID]*domain.Form
}

func newMockFormRepo() *mockFormRepo {
	return &mockFormRepo{forms: make(map[uuid.UUID]*domain.Form)}
}

func (m *mockFormRepo) DB() *sql.DB { return nil }
func (m *mockFormRepo) Save(ctx context.Context, tx *sql.Tx, form *domain.Form) error {
	m.forms[form.ID] = form
	return nil
}
func (m *mockFormRepo) SaveTx(ctx context.Context, tx *sql.Tx, form *domain.Form) error {
	return m.Save(ctx, tx, form)
}
func (m *mockFormRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Form, error) {
	form, ok := m.forms[id]
	if !ok {
		return nil, domain.ErrFormNotFound
	}
	return form, nil
}
func (m *mockFormRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*domain.Form, error) {
	return m.FindByID(ctx, orgID, id)
}

func (m *mockFormRepo) FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*domain.Form, error) {
	return m.FindByIDTx(ctx, tx, orgID, id)
}
func (m *mockFormRepo) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.Form, error) {
	return nil, domain.ErrFormNotFound
}
func (m *mockFormRepo) FindByKeyTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, key string) (*domain.Form, error) {
	return nil, domain.ErrFormNotFound
}
func (m *mockFormRepo) List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Form, int, error) {
	return nil, 0, nil
}
func (m *mockFormRepo) Update(ctx context.Context, orgID uuid.UUID, id uuid.UUID, form *domain.Form) error {
	m.forms[id] = form
	return nil
}
func (m *mockFormRepo) UpdateTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, id uuid.UUID, form *domain.Form) error {
	return m.Update(ctx, orgID, id, form)
}

func (m *mockFormRepo) FindByIDs(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) ([]*domain.Form, error) {
	var result []*domain.Form
	for _, f := range m.forms {
		for _, id := range ids {
			if f.ID == id {
				result = append(result, f)
				break
			}
		}
	}
	return result, nil
}

type mockFormVersionRepo struct {
	versions map[uuid.UUID]*domain.FormVersion
}

func newMockFormVersionRepo() *mockFormVersionRepo {
	return &mockFormVersionRepo{versions: make(map[uuid.UUID]*domain.FormVersion)}
}

func (m *mockFormVersionRepo) DB() *sql.DB { return nil }
func (m *mockFormVersionRepo) Save(ctx context.Context, tx *sql.Tx, version *domain.FormVersion) error {
	m.versions[version.ID] = version
	return nil
}
func (m *mockFormVersionRepo) SaveTx(ctx context.Context, tx *sql.Tx, version *domain.FormVersion) error {
	return m.Save(ctx, tx, version)
}
func (m *mockFormVersionRepo) FindLatest(ctx context.Context, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}
func (m *mockFormVersionRepo) FindLatestTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}
func (m *mockFormVersionRepo) FindActiveVersion(ctx context.Context, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}
func (m *mockFormVersionRepo) FindActiveVersionTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}
func (m *mockFormVersionRepo) FindByID(ctx context.Context, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	version, ok := m.versions[versionID]
	if !ok {
		return nil, domain.ErrFormVersionNotFound
	}
	return version, nil
}
func (m *mockFormVersionRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	return m.FindByID(ctx, orgID, versionID)
}

func (m *mockFormVersionRepo) FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	return m.FindByIDTx(ctx, tx, orgID, versionID)
}

func (m *mockFormVersionRepo) FindByIDs(ctx context.Context, orgID uuid.UUID, versionIDs []uuid.UUID) ([]*domain.FormVersion, error) {
	var result []*domain.FormVersion
	for _, v := range m.versions {
		for _, id := range versionIDs {
			if v.ID == id {
				result = append(result, v)
				break
			}
		}
	}
	return result, nil
}

func (m *mockFormVersionRepo) ListByFormID(ctx context.Context, orgID, formID uuid.UUID) ([]*domain.FormVersion, error) {
	return nil, nil
}

type mockAssignmentRepo struct {
	assignments map[uuid.UUID]*assignmentdomain.WorkflowStateFormAssignment
}

func newMockAssignmentRepo() *mockAssignmentRepo {
	return &mockAssignmentRepo{assignments: make(map[uuid.UUID]*assignmentdomain.WorkflowStateFormAssignment)}
}

func (m *mockAssignmentRepo) DB() *sql.DB { return nil }
func (m *mockAssignmentRepo) Save(ctx context.Context, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	m.assignments[assignment.ID] = assignment
	return nil
}
func (m *mockAssignmentRepo) SaveTx(ctx context.Context, tx *sql.Tx, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	return m.Save(ctx, assignment)
}
func (m *mockAssignmentRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	a, ok := m.assignments[id]
	if !ok {
		return nil, assignmentdomain.ErrAssignmentNotFound
	}
	return a, nil
}
func (m *mockAssignmentRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	return m.FindByID(ctx, tenantID, id)
}
func (m *mockAssignmentRepo) FindByWorkflowAndState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	var result []*assignmentdomain.WorkflowStateFormAssignment
	for _, a := range m.assignments {
		if a.TenantID == tenantID && a.WorkflowDefinitionID == workflowDefID && a.WorkflowStateKey == stateKey {
			result = append(result, a)
		}
	}
	return result, nil
}
func (m *mockAssignmentRepo) FindByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	return m.FindByWorkflowAndState(ctx, tenantID, workflowDefID, stateKey)
}
func (m *mockAssignmentRepo) ListByWorkflow(ctx context.Context, tenantID, workflowDefID uuid.UUID) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	var result []*assignmentdomain.WorkflowStateFormAssignment
	for _, a := range m.assignments {
		if a.TenantID == tenantID && a.WorkflowDefinitionID == workflowDefID {
			result = append(result, a)
		}
	}
	return result, nil
}
func (m *mockAssignmentRepo) Update(ctx context.Context, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	m.assignments[assignment.ID] = assignment
	return nil
}
func (m *mockAssignmentRepo) UpdateTx(ctx context.Context, tx *sql.Tx, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	return m.Update(ctx, assignment)
}
func (m *mockAssignmentRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	delete(m.assignments, id)
	return nil
}
func (m *mockAssignmentRepo) DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error {
	return m.Delete(ctx, tenantID, id)
}
func (m *mockAssignmentRepo) DeleteByWorkflowAndState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) error {
	return nil
}
func (m *mockAssignmentRepo) DeleteByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) error {
	return nil
}

func (m *mockAssignmentRepo) FindByWorkflowAndStateForUpdateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	return m.FindByWorkflowAndState(ctx, tenantID, workflowDefID, stateKey)
}

type mockAuditor struct {
	events []auditdomain.RecordEventParams
}

func (m *mockAuditor) RecordEvent(ctx context.Context, params auditdomain.RecordEventParams) error {
	m.events = append(m.events, params)
	return nil
}

func TestCreateAssignment_Success(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()
	formID := uuid.New()
	formVersionID := uuid.New()
	actorID := uuid.New()

	defRepo := newMockWorkflowDefRepo()
	formRepo := newMockFormRepo()
	versionRepo := newMockFormVersionRepo()
	assignmentRepo := newMockAssignmentRepo()
	auditor := &mockAuditor{}

	workflowDef := &workflowdomain.WorkflowDefinition{
		ID:       workflowDefID,
		TenantID: orgID,
		Key:      "test-workflow",
		Name:     "Test Workflow",
		States: []workflowdomain.WorkflowState{
			{Key: "OPEN", Name: "Open"},
			{Key: "ASSESSMENT", Name: "Assessment"},
		},
	}
	defRepo.defs[workflowDefID] = workflowDef

	form := &domain.Form{
		ID:             formID,
		OrganizationID: orgID,
		Key:            "test-form",
		Name:           "Test Form",
		Status:         domain.FormStatusActive,
	}
	formRepo.forms[formID] = form

	version := &domain.FormVersion{
		ID:             formVersionID,
		FormID:         formID,
		OrganizationID: orgID,
		Version:        1,
		Status:         domain.FormVersionStatusPublished,
	}
	versionRepo.versions[formVersionID] = version

	svc := NewWorkflowStateFormAssignmentService(defRepo, formRepo, versionRepo, assignmentRepo, auditor)

	assignment, err := svc.CreateAssignment(context.Background(), CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDefID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               formID,
		FormVersionID:        formVersionID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "ASSESSMENT", assignment.WorkflowStateKey)
	assert.Equal(t, formID, assignment.FormID)
	assert.Equal(t, formVersionID, assignment.FormVersionID)
	assert.True(t, assignment.Required)
	assert.True(t, assignment.Active)
}

func TestCreateAssignment_InvalidState(t *testing.T) {
	svc := NewWorkflowStateFormAssignmentService(newMockWorkflowDefRepo(), newMockFormRepo(), newMockFormVersionRepo(), newMockAssignmentRepo(), &mockAuditor{})

	_, err := svc.CreateAssignment(context.Background(), CreateAssignmentParams{
		TenantID:             uuid.New(),
		WorkflowDefinitionID: uuid.New(),
		WorkflowStateKey:     "",
		FormID:               uuid.New(),
		FormVersionID:        uuid.New(),
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            uuid.New(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidWorkflowState)
}

func TestCreateAssignment_DuplicateAssignment(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()
	formID := uuid.New()
	formVersionID := uuid.New()
	actorID := uuid.New()

	defRepo := newMockWorkflowDefRepo()
	formRepo := newMockFormRepo()
	versionRepo := newMockFormVersionRepo()
	assignmentRepo := newMockAssignmentRepo()
	auditor := &mockAuditor{}

	workflowDef := &workflowdomain.WorkflowDefinition{
		ID:       workflowDefID,
		TenantID: orgID,
		Key:      "test-workflow",
		Name:     "Test Workflow",
		States: []workflowdomain.WorkflowState{
			{Key: "ASSESSMENT", Name: "Assessment"},
		},
	}
	defRepo.defs[workflowDefID] = workflowDef

	form := &domain.Form{ID: formID, OrganizationID: orgID, Key: "test-form", Name: "Test Form", Status: domain.FormStatusActive}
	formRepo.forms[formID] = form

	version := &domain.FormVersion{ID: formVersionID, FormID: formID, OrganizationID: orgID, Version: 1, Status: domain.FormVersionStatusPublished}
	versionRepo.versions[formVersionID] = version

	existingAssignment, _ := assignmentdomain.NewWorkflowStateFormAssignment(orgID, workflowDefID, formID, formVersionID, actorID, "ASSESSMENT", true, 0)
	assignmentRepo.assignments[existingAssignment.ID] = existingAssignment

	svc := NewWorkflowStateFormAssignmentService(defRepo, formRepo, versionRepo, assignmentRepo, auditor)

	_, err := svc.CreateAssignment(context.Background(), CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDefID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               formID,
		FormVersionID:        formVersionID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAssignmentAlreadyExists)
}

func TestCreateAssignment_FormNotPublished(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()
	formID := uuid.New()
	formVersionID := uuid.New()
	actorID := uuid.New()

	defRepo := newMockWorkflowDefRepo()
	formRepo := newMockFormRepo()
	versionRepo := newMockFormVersionRepo()
	assignmentRepo := newMockAssignmentRepo()

	workflowDef := &workflowdomain.WorkflowDefinition{
		ID:       workflowDefID,
		TenantID: orgID,
		Key:      "test-workflow",
		Name:     "Test Workflow",
		States: []workflowdomain.WorkflowState{
			{Key: "ASSESSMENT", Name: "Assessment"},
		},
	}
	defRepo.defs[workflowDefID] = workflowDef

	form := &domain.Form{ID: formID, OrganizationID: orgID, Key: "test-form", Name: "Test Form", Status: domain.FormStatusActive}
	formRepo.forms[formID] = form

	version := &domain.FormVersion{ID: formVersionID, FormID: formID, OrganizationID: orgID, Version: 1, Status: domain.FormVersionStatusDraft}
	versionRepo.versions[formVersionID] = version

	svc := NewWorkflowStateFormAssignmentService(defRepo, formRepo, versionRepo, assignmentRepo, nil)

	_, err := svc.CreateAssignment(context.Background(), CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDefID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               formID,
		FormVersionID:        formVersionID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidFormVersion)
}

func TestCreateAssignment_FormArchived(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()
	formID := uuid.New()
	formVersionID := uuid.New()
	actorID := uuid.New()

	defRepo := newMockWorkflowDefRepo()
	formRepo := newMockFormRepo()
	versionRepo := newMockFormVersionRepo()
	assignmentRepo := newMockAssignmentRepo()

	workflowDef := &workflowdomain.WorkflowDefinition{
		ID:       workflowDefID,
		TenantID: orgID,
		Key:      "test-workflow",
		Name:     "Test Workflow",
		States: []workflowdomain.WorkflowState{
			{Key: "ASSESSMENT", Name: "Assessment"},
		},
	}
	defRepo.defs[workflowDefID] = workflowDef

	form := &domain.Form{ID: formID, OrganizationID: orgID, Key: "test-form", Name: "Test Form", Status: domain.FormStatusArchived}
	formRepo.forms[formID] = form

	version := &domain.FormVersion{ID: formVersionID, FormID: formID, OrganizationID: orgID, Version: 1, Status: domain.FormVersionStatusPublished}
	versionRepo.versions[formVersionID] = version

	svc := NewWorkflowStateFormAssignmentService(defRepo, formRepo, versionRepo, assignmentRepo, nil)

	_, err := svc.CreateAssignment(context.Background(), CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDefID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               formID,
		FormVersionID:        formVersionID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFormArchived)
}

func TestUpdateAssignment_Success(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()
	formID := uuid.New()
	formVersionID := uuid.New()
	actorID := uuid.New()

	defRepo := newMockWorkflowDefRepo()
	formRepo := newMockFormRepo()
	versionRepo := newMockFormVersionRepo()
	assignmentRepo := newMockAssignmentRepo()
	auditor := &mockAuditor{}

	assignment, _ := assignmentdomain.NewWorkflowStateFormAssignment(orgID, workflowDefID, formID, formVersionID, actorID, "OPEN", true, 0)
	assignmentRepo.assignments[assignment.ID] = assignment

	svc := NewWorkflowStateFormAssignmentService(defRepo, formRepo, versionRepo, assignmentRepo, auditor)

	updated, err := svc.UpdateAssignment(context.Background(), UpdateAssignmentParams{
		TenantID:     orgID,
		AssignmentID: assignment.ID,
		Required:     boolPtr(false),
		Active:       boolPtr(false),
	})
	require.NoError(t, err)
	assert.False(t, updated.Required)
	assert.False(t, updated.Active)
}

func TestDeleteAssignment_Success(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()
	formID := uuid.New()
	formVersionID := uuid.New()
	actorID := uuid.New()

	assignmentRepo := newMockAssignmentRepo()
	auditor := &mockAuditor{}

	assignment, _ := assignmentdomain.NewWorkflowStateFormAssignment(orgID, workflowDefID, formID, formVersionID, actorID, "OPEN", true, 0)
	assignmentRepo.assignments[assignment.ID] = assignment

	svc := NewWorkflowStateFormAssignmentService(newMockWorkflowDefRepo(), newMockFormRepo(), newMockFormVersionRepo(), assignmentRepo, auditor)

	err := svc.DeleteAssignment(context.Background(), orgID, assignment.ID, actorID)
	require.NoError(t, err)

	_, err = assignmentRepo.FindByID(context.Background(), orgID, assignment.ID)
	require.Error(t, err)
}

func TestListAssignmentsByWorkflow_Success(t *testing.T) {
	orgID := uuid.New()
	workflowDefID := uuid.New()

	defRepo := newMockWorkflowDefRepo()
	assignmentRepo := newMockAssignmentRepo()

	workflowDef := &workflowdomain.WorkflowDefinition{
		ID:       workflowDefID,
		TenantID: orgID,
		Key:      "test-workflow",
		Name:     "Test Workflow",
		States:   []workflowdomain.WorkflowState{},
	}
	defRepo.defs[workflowDefID] = workflowDef

	svc := NewWorkflowStateFormAssignmentService(defRepo, newMockFormRepo(), newMockFormVersionRepo(), assignmentRepo, nil)

	assignments, err := svc.ListAssignmentsByWorkflow(context.Background(), orgID, workflowDefID)
	require.NoError(t, err)
	assert.Empty(t, assignments)
}

func boolPtr(b bool) *bool {
	return &b
}
