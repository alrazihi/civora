package integration

import (
	"context"
	"database/sql"
	"testing"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	formapp "github.com/alrazihi/civora/internal/forms/application"
	formdomain "github.com/alrazihi/civora/internal/forms/domain"
	formpostgres "github.com/alrazihi/civora/internal/forms/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFormServices(t *testing.T, db *sql.DB) (*formapp.FormService, uuid.UUID) {
	t.Helper()
	orgID := helpers.SeedOrg(db)

	formRepo := formpostgres.NewPostgresFormRepository(db)
	versionRepo := formpostgres.NewPostgresFormVersionRepository(db)
	fieldRepo := formpostgres.NewPostgresFormFieldRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	svc := formapp.NewFormService(formRepo, versionRepo, fieldRepo, auditService)

	return svc, orgID
}

func TestForm_CRUDLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "contact-form",
		Name:           "Contact Form",
		Description:    "Customer contact form",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	require.NotNil(t, form)
	assert.Equal(t, "contact-form", form.Key)
	assert.Equal(t, "Contact Form", form.Name)
	assert.Equal(t, formdomain.FormStatusDraft, form.Status)
	assert.Equal(t, orgID, form.OrganizationID)

	fetched, err := svc.GetForm(ctx, formapp.GetFormParams{
		OrganizationID: orgID,
		FormID:         form.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, form.ID, fetched.ID)
	assert.Equal(t, "contact-form", fetched.Key)

	updated, err := svc.UpdateForm(ctx, formapp.UpdateFormParams{
		OrganizationID: orgID,
		ID:             form.ID,
		Name:           "Updated Contact Form",
		Description:    "Updated description",
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Contact Form", updated.Name)
	assert.Equal(t, "Updated description", updated.Description)

	forms, total, err := svc.ListForms(ctx, formapp.ListFormsParams{
		OrganizationID: orgID,
		Offset:         0,
		Limit:          10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, forms, 1)
	assert.Equal(t, form.ID, forms[0].ID)

	archived, err := svc.ArchiveForm(ctx, formapp.ArchiveFormParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, formdomain.FormStatusArchived, archived.Status)
}

func TestForm_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	ctx := context.Background()

	svc1, org1 := setupFormServices(t, db)
	svc2, org2 := setupFormServices(t, db)
	actor1 := helpers.SeedUser(db, org1)
	actor2 := helpers.SeedUser(db, org2)

	form, err := svc1.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: org1,
		Key:            "tenant-a-form",
		Name:           "Tenant A Form",
		CreatedByID:    actor1,
	})
	require.NoError(t, err)

	_, err = svc2.GetForm(ctx, formapp.GetFormParams{
		OrganizationID: org2,
		FormID:         form.ID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "form not found")

	_, err = svc2.UpdateForm(ctx, formapp.UpdateFormParams{
		OrganizationID: org2,
		ID:             form.ID,
		Name:           "Hacked",
		ActorID:        actor2,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "form not found")

	_, err = svc2.ArchiveForm(ctx, formapp.ArchiveFormParams{
		OrganizationID: org2,
		FormID:         form.ID,
		ActorID:        actor2,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "form not found")

	forms, total, err := svc2.ListForms(ctx, formapp.ListFormsParams{
		OrganizationID: org2,
		Offset:         0,
		Limit:          10,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, forms)
}

func TestForm_FieldCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "survey",
		Name:           "Survey",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	version, err := svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, version.Version)
	assert.Equal(t, formdomain.FormVersionStatusDraft, version.Status)

	field, err := svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "full_name",
		Label:          "Full Name",
		Type:           formdomain.FieldTypeText,
		Required:       true,
		Placeholder:    "Enter your name",
		Validation: map[string]any{
			"minLength": 2,
			"maxLength": 100,
		},
		ActorID: actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "full_name", field.Key)
	assert.Equal(t, "Full Name", field.Label)
	assert.Equal(t, formdomain.FieldTypeText, field.Type)
	assert.True(t, field.Required)
	assert.Equal(t, "Enter your name", field.Placeholder)

	updatedField, err := svc.UpdateField(ctx, formapp.UpdateFieldParams{
		OrganizationID: orgID,
		FieldID:        field.ID,
		Label:          "Legal Name",
		Required:       false,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Legal Name", updatedField.Label)
	assert.False(t, updatedField.Required)

	deleted, err := svc.DeleteField(ctx, formapp.DeleteFieldParams{
		OrganizationID: orgID,
		FieldID:        field.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, field.ID, deleted.ID)

	fetched, err := svc.GetForm(ctx, formapp.GetFormParams{
		OrganizationID: orgID,
		FormID:         form.ID,
	})
	require.NoError(t, err)
	assert.Empty(t, fetched.Versions[0].Fields)
}

func TestForm_VersionLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "multi-ver",
		Name:           "Multi Version",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	v1, err := svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, v1.Version)

	v2, err := svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, v2.Version)

	published, err := svc.PublishVersion(ctx, formapp.PublishVersionParams{
		OrganizationID: orgID,
		VersionID:      v2.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, formdomain.FormVersionStatusPublished, published.Status)

	active, err := svc.GetActiveVersion(ctx, formapp.GetActiveVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, v2.ID, active.ID)
	assert.Equal(t, 2, active.Version)

	_, err = svc.PublishVersion(ctx, formapp.PublishVersionParams{
		OrganizationID: orgID,
		VersionID:      v1.ID,
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid version status")
}

func TestForm_InvalidFieldType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "test-form",
		Name:           "Test",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	version, err := svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "bad",
		Label:          "Bad Field",
		Type:           "INVALID_TYPE",
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid field type")
}

func TestForm_DuplicateFieldKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "dup-test",
		Name:           "Dup Test",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	version, err := svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "field-a",
		Label:          "Field A",
		Type:           formdomain.FieldTypeText,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "field-a",
		Label:          "Field A Duplicate",
		Type:           formdomain.FieldTypeText,
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "duplicate field key")
}

func TestForm_AuditEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "audit-test",
		Name:           "Audit Test",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	version, err := svc.GetActiveVersion(ctx, formapp.GetActiveVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
	})
	require.NoError(t, err)

	_, err = svc.PublishVersion(ctx, formapp.PublishVersionParams{
		OrganizationID: orgID,
		VersionID:      version.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = svc.ArchiveForm(ctx, formapp.ArchiveFormParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)
}

func TestForm_ValidationErrorMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "validation-meta",
		Name:           "Validation Meta",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	version, err := svc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	options := []formdomain.FormOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
	}

	field, err := svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "rating",
		Label:          "Rating",
		Type:           formdomain.FieldTypeSelect,
		Required:       true,
		Options:        options,
		Validation: map[string]any{
			"minValue": 1,
			"maxValue": 10,
		},
		ActorID: actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, formdomain.FieldTypeSelect, field.Type)
	assert.Equal(t, options, field.Options)
	assert.Equal(t, int64(1), field.Validation["minValue"])
	assert.Equal(t, int64(10), field.Validation["maxValue"])

	_, err = svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "email-field",
		Label:          "Email",
		Type:           formdomain.FieldTypeEmail,
		Required:       true,
		Validation: map[string]any{
			"maxLength": "not-a-number",
		},
		ActorID: actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid input")
}

func TestForm_TransactionRollbackOnInvalidVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	svc, orgID := setupFormServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	form, err := svc.CreateForm(ctx, formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "tx-test",
		Name:           "Tx Test",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = svc.AddField(ctx, formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      uuid.Nil,
		Key:            "should-rollback",
		Label:          "Should Rollback",
		Type:           formdomain.FieldTypeText,
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "form version not found")

	fetched, err := svc.GetForm(ctx, formapp.GetFormParams{
		OrganizationID: orgID,
		FormID:         form.ID,
	})
	require.NoError(t, err)
	for _, v := range fetched.Versions {
		assert.Empty(t, v.Fields)
	}
}
