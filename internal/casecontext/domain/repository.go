package domain

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type CaseContextRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, c *CaseContext) error
	SaveTx(ctx context.Context, tx *sql.Tx, c *CaseContext) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*CaseContext, error)
	FindLatestByCase(ctx context.Context, orgID, caseID uuid.UUID) (*CaseContext, error)
	DeleteExpired(ctx context.Context, orgID uuid.UUID) (int, error)
}

type AuthorizationChecker interface {
	CanAccessCase(ctx context.Context, orgID, caseID, actorID uuid.UUID) (bool, error)
	CanAccessPerson(ctx context.Context, orgID, personID, actorID uuid.UUID) (bool, error)
	CanAccessEvidence(ctx context.Context, orgID, evidenceID, actorID uuid.UUID) (bool, error)
	CanAccessDocument(ctx context.Context, orgID, documentID, actorID uuid.UUID) (bool, error)
	CanAccessFormSubmission(ctx context.Context, orgID, submissionID, actorID uuid.UUID) (bool, error)
	CanAccessRuleEvaluation(ctx context.Context, orgID, evaluationID, actorID uuid.UUID) (bool, error)
	CanAccessDecision(ctx context.Context, orgID, decisionID, actorID uuid.UUID) (bool, error)
}

type CaseRepository interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Case, error)
}

type Case struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	CaseNumber         string
	Title              string
	Description        string
	Status             string
	WorkflowState      string
	ServiceType        string
	Priority           string
	PersonID           *uuid.UUID
	AssignedToID       *uuid.UUID
	CreatedByID        uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ClosedAt           *time.Time
	WorkflowInstanceID *uuid.UUID
	WorkflowKey        string
	WorkflowID         *uuid.UUID
	Version            int
}

type PersonRepository interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Person, error)
}

type Person struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	LegalName      string
	DateOfBirth    *time.Time
	Gender         string
	Nationality    string
	Address        string
	Phone          string
	Email          string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type EvidenceRepository interface {
	FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*Evidence, int, error)
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Evidence, error)
	ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*Document, int, error)
}

type Evidence struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	ServiceRequestID   uuid.UUID
	Type               string
	CustomType         *string
	Description        string
	StorageReference   string
	UploadedBy         uuid.UUID
	PersonID           *uuid.UUID
	Source             string
	Metadata           map[string]any
	VerificationStatus string
	VerifiedBy         *uuid.UUID
	VerifiedAt         *time.Time
	VerificationReason string
	VerificationMethod string
	CreatedAt          time.Time
}

type Document struct {
	ID              uuid.UUID
	EvidenceID      uuid.UUID
	OrganizationID  uuid.UUID
	FileName        string
	ContentType     string
	SizeBytes       int64
	Checksum        string
	StorageKey      string
	StorageProvider string
	UploadedBy      uuid.UUID
	UploadedAt      time.Time
	CreatedAt       time.Time
}

type DocumentContentProvider interface {
	GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) ([]byte, error)
}

type FormSubmissionRepository interface {
	ListByCase(ctx context.Context, orgID, caseID uuid.UUID) ([]*FormSubmission, error)
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*FormSubmission, error)
}

type FormSubmission struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	CaseID        uuid.UUID
	FormID        uuid.UUID
	FormVersionID uuid.UUID
	SubmittedBy   uuid.UUID
	Status        string
	Data          map[string]interface{}
	SubmittedAt   time.Time
	UpdatedAt     time.Time
}

type RuleEvaluationRepository interface {
	FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*RuleEvaluation, int, error)
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*RuleEvaluation, error)
}

type RuleEvaluation struct {
	ID             uuid.UUID
	RuleSetID      uuid.UUID
	RuleSetVersion int
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	Status         string
	Outcome        string
	Reason         *string
	MatchedRuleID  *uuid.UUID
	Trace          []TraceNode
	Trigger        string
	EvaluatedBy    *uuid.UUID
	EvaluatedAt    time.Time
	FactsSnapshot  map[string]interface{}
}

type TraceNode struct {
	ID          string
	NodeType    string
	Description string
	Field       string
	Operator    string
	Expected    []byte
	Actual      []byte
	ActualType  string
	Result      string
	Reason      string
	Children    []TraceNode
	Timestamp   time.Time
}

type WorkflowRepository interface {
	FindInstanceByCase(ctx context.Context, orgID, caseID uuid.UUID) (*WorkflowInstance, error)
	FindHistoryByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*WorkflowTransitionHistory, int, error)
}

type WorkflowInstance struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	WorkflowDefID      uuid.UUID
	WorkflowDefVersion int
	CaseID             uuid.UUID
	CurrentState       string
	StartedAt          time.Time
	CompletedAt        *time.Time
	Metadata           map[string]interface{}
	Version            int
}

type WorkflowTransitionHistory struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	WorkflowInstanceID uuid.UUID
	CaseID             uuid.UUID
	FromState          string
	ToState            string
	TransitionKey      string
	ActorID            *uuid.UUID
	OccurredAt         time.Time
	Reason             string
	Metadata           map[string]interface{}
	DecisionID         *uuid.UUID
}

type DecisionRepository interface {
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*Decision, int, error)
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Decision, error)
}

type Decision struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	ServiceRequestID   uuid.UUID
	Decision           string
	Reason             string
	DecisionMaker      uuid.UUID
	DecidedAt          time.Time
	CreatedAt          time.Time
	SupersededByID     *uuid.UUID
	WorkflowState      string
	RuleEvaluationIDs  []uuid.UUID
	EvidenceIDs        []uuid.UUID
	FormSubmissionID   *uuid.UUID
	ReviewQueueEntryID *uuid.UUID
	Version            int
}

type AIObservationRepository interface {
	FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*AIObservation, int, error)
}

type AIObservation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	Type           string
	Source         string
	Status         string
	Model          *ModelInfo
	Content        map[string]any
	Confidence     *float64
	InputHash      string
	OutputHash     string
	CreatedAt      time.Time
	CreatedBy      *uuid.UUID
	ReviewedAt     *time.Time
	ReviewedBy     *uuid.UUID
	ReviewNotes    string
}

type ModelInfo struct {
	Name     string
	Version  string
	Provider string
}
