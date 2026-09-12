package application

import (
	"context"
	"database/sql"
	"testing"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFormRepo struct {
	forms map[string]*domain.Form
}

func newMockFormRepo() *mockFormRepo {
	return &mockFormRepo{forms: make(map[string]*domain.Form)}
}

func (m *mockFormRepo) Save(ctx context.Context, tx *sql.Tx, form *domain.Form) error {
	return m.SaveTx(ctx, tx, form)
}

func (m *mockFormRepo) DB() *sql.DB { return nil }

func (m *mockFormRepo) SaveTx(ctx context.Context, tx *sql.Tx, form *domain.Form) error {
	m.forms[form.Key] = form
	return nil
}

func (m *mockFormRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Form, error) {
	for _, f := range m.forms {
		if f.ID == id {
			return f, nil
		}
	}
	return nil, domain.ErrFormNotFound
}

func (m *mockFormRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*domain.Form, error) {
	return m.FindByID(ctx, orgID, id)
}

func (m *mockFormRepo) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.Form, error) {
	f, ok := m.forms[key]
	if !ok {
		return nil, domain.ErrFormNotFound
	}
	return f, nil
}

func (m *mockFormRepo) FindByKeyTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, key string) (*domain.Form, error) {
	return m.FindByKey(ctx, orgID, key)
}

func (m *mockFormRepo) List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Form, int, error) {
	return nil, 0, nil
}

func (m *mockFormRepo) Update(ctx context.Context, orgID, id uuid.UUID, form *domain.Form) error {
	return nil
}

func (m *mockFormRepo) UpdateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, form *domain.Form) error {
	return nil
}

type mockVersionRepo struct {
	versions map[string]*domain.FormVersion
}

func newMockVersionRepo() *mockVersionRepo {
	return &mockVersionRepo{versions: make(map[string]*domain.FormVersion)}
}

func (m *mockVersionRepo) Save(ctx context.Context, tx *sql.Tx, version *domain.FormVersion) error {
	return m.SaveTx(ctx, tx, version)
}

func (m *mockVersionRepo) SaveTx(ctx context.Context, tx *sql.Tx, version *domain.FormVersion) error {
	m.versions[version.ID.String()] = version
	return nil
}

func (m *mockVersionRepo) FindLatest(ctx context.Context, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}

func (m *mockVersionRepo) FindLatestTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}

func (m *mockVersionRepo) FindActiveVersion(ctx context.Context, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}

func (m *mockVersionRepo) FindActiveVersionTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return nil, domain.ErrFormVersionNotFound
}

func (m *mockVersionRepo) FindByID(ctx context.Context, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	v, ok := m.versions[versionID.String()]
	if !ok {
		return nil, domain.ErrFormVersionNotFound
	}
	return v, nil
}

func (m *mockVersionRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	return m.FindByID(ctx, orgID, versionID)
}

func (m *mockVersionRepo) ListByFormID(ctx context.Context, orgID, formID uuid.UUID) ([]*domain.FormVersion, error) {
	return nil, nil
}

type mockFieldRepo struct {
	fields map[string]*domain.FormField
}

func newMockFieldRepo() *mockFieldRepo {
	return &mockFieldRepo{fields: make(map[string]*domain.FormField)}
}

func (m *mockFieldRepo) Save(ctx context.Context, tx *sql.Tx, field *domain.FormField) error {
	return m.SaveTx(ctx, tx, field)
}

func (m *mockFieldRepo) SaveTx(ctx context.Context, tx *sql.Tx, field *domain.FormField) error {
	m.fields[field.ID.String()] = field
	return nil
}

func (m *mockFieldRepo) FindByVersion(ctx context.Context, orgID, versionID uuid.UUID) ([]*domain.FormField, error) {
	return nil, nil
}

func (m *mockFieldRepo) FindByVersionTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) ([]*domain.FormField, error) {
	return nil, nil
}

func (m *mockFieldRepo) FindByID(ctx context.Context, orgID, fieldID uuid.UUID) (*domain.FormField, error) {
	f, ok := m.fields[fieldID.String()]
	if !ok {
		return nil, domain.ErrFormFieldNotFound
	}
	return f, nil
}

func (m *mockFieldRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, fieldID uuid.UUID) (*domain.FormField, error) {
	return m.FindByID(ctx, orgID, fieldID)
}

func (m *mockFieldRepo) UpdateTx(ctx context.Context, tx *sql.Tx, field *domain.FormField) error {
	m.fields[field.ID.String()] = field
	return nil
}

func (m *mockFieldRepo) DeleteTx(ctx context.Context, tx *sql.Tx, orgID, fieldID uuid.UUID) error {
	delete(m.fields, fieldID.String())
	return nil
}

func (m *mockFieldRepo) UpdateOrderTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID, fields []*domain.FormField) error {
	return nil
}

type mockAuditor struct {
	events []auditdomain.RecordEventParams
}

func (m *mockAuditor) RecordEvent(ctx context.Context, params auditdomain.RecordEventParams) error {
	m.events = append(m.events, params)
	return nil
}

func TestCreateForm_ValidInput(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	ctx := context.Background()
	orgID := uuid.New()
	actorID := uuid.New()

	form, err := svc.CreateForm(ctx, CreateFormParams{
		OrganizationID: orgID,
		Key:            "my-form",
		Name:           "My Form",
		Description:    "Test form",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "my-form", form.Key)
	assert.Equal(t, "My Form", form.Name)
	assert.Equal(t, domain.FormStatusDraft, form.Status)
	assert.Equal(t, orgID, form.OrganizationID)
	assert.Equal(t, actorID, form.CreatedByID)
}

func TestCreateForm_EmptyKey(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.CreateForm(context.Background(), CreateFormParams{
		OrganizationID: uuid.New(),
		Key:            "",
		Name:           "My Form",
		CreatedByID:    uuid.New(),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "key is required")
}

func TestCreateForm_EmptyName(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.CreateForm(context.Background(), CreateFormParams{
		OrganizationID: uuid.New(),
		Key:            "my-form",
		Name:           "",
		CreatedByID:    uuid.New(),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "name is required")
}

func TestAddField_ValidInput(t *testing.T) {
	orgID := uuid.New()

	form, _ := domain.NewForm(orgID, "my-form", "My Form", "", uuid.New())
	repo := newMockFormRepo()
	_ = repo.SaveTx(context.Background(), nil, form)

	version := domain.NewFormVersion(form.ID, orgID, uuid.New())
	version.Version = 1
	vRepo := newMockVersionRepo()
	_ = vRepo.SaveTx(context.Background(), nil, version)

	fieldRepo := newMockFieldRepo()

	svc := NewFormService(repo, vRepo, fieldRepo, &mockAuditor{})
	field, err := svc.AddField(context.Background(), AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "name",
		Label:          "Full Name",
		Type:           domain.FieldTypeText,
		Required:       true,
		ActorID:        uuid.New(),
	})
	require.NoError(t, err)
	assert.Equal(t, "name", field.Key)
	assert.Equal(t, "Full Name", field.Label)
	assert.Equal(t, domain.FieldTypeText, field.Type)
	assert.True(t, field.Required)
}

func TestAddField_InvalidType(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.AddField(context.Background(), AddFieldParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
		VersionID:      uuid.New(),
		Key:            "f",
		Label:          "Field",
		Type:           domain.FieldType("INVALID"),
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestAddField_EmptyKey(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.AddField(context.Background(), AddFieldParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
		VersionID:      uuid.New(),
		Key:            "",
		Label:          "Field",
		Type:           domain.FieldTypeText,
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestAddField_EmptyLabel(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.AddField(context.Background(), AddFieldParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
		VersionID:      uuid.New(),
		Key:            "f",
		Label:          "",
		Type:           domain.FieldTypeText,
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestCreateVersion(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.CreateVersion(context.Background(), CreateVersionParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestPublishVersion(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.PublishVersion(context.Background(), PublishVersionParams{
		OrganizationID: uuid.New(),
		VersionID:      uuid.New(),
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestGetForm_NotFound(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.GetForm(context.Background(), GetFormParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
	})
	require.Error(t, err)
}

func TestListForms(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	forms, total, err := svc.ListForms(context.Background(), ListFormsParams{
		OrganizationID: uuid.New(),
		Limit:          20,
		Offset:         0,
	})
	require.NoError(t, err)
	assert.Nil(t, forms)
	assert.Equal(t, 0, total)
}

func TestUpdateForm(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.UpdateForm(context.Background(), UpdateFormParams{
		OrganizationID: uuid.New(),
		ID:             uuid.New(),
		Name:           "Updated Name",
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestArchiveForm(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.ArchiveForm(context.Background(), ArchiveFormParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestDeleteField(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.DeleteField(context.Background(), DeleteFieldParams{
		OrganizationID: uuid.New(),
		FieldID:        uuid.New(),
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}

func TestGetActiveVersion(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.GetActiveVersion(context.Background(), GetActiveVersionParams{
		OrganizationID: uuid.New(),
		FormID:         uuid.New(),
	})
	require.Error(t, err)
}

func TestUpdateField(t *testing.T) {
	svc := NewFormService(newMockFormRepo(), newMockVersionRepo(), newMockFieldRepo(), &mockAuditor{})
	_, err := svc.UpdateField(context.Background(), UpdateFieldParams{
		OrganizationID: uuid.New(),
		FieldID:        uuid.New(),
		Label:          "Updated",
		ActorID:        uuid.New(),
	})
	require.Error(t, err)
}
