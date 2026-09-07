package shared

import (
	"context"
	"database/sql"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
)

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func RecordAuditEventInTx(ctx context.Context, tx *sql.Tx, auditor auditdomain.EventRecorder, params auditdomain.RecordEventParams) error {
	if txRecorder, ok := auditor.(txEventRecorder); ok {
		return txRecorder.RecordEventInTx(ctx, tx, params)
	}
	return auditor.RecordEvent(ctx, params)
}

func StrPtr(s string) *string {
	return &s
}
