package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowKeyForServiceType(t *testing.T) {
	tests := []struct {
		serviceType ServiceType
		expected    string
	}{
		{ServiceTypeEmergency, "emergency_assistance"},
		{ServiceTypeMedical, "medical_assistance"},
		{ServiceTypeFinancial, "financial_assistance"},
		{ServiceTypeFood, "food_assistance"},
		{ServiceTypeShelter, "shelter_assistance"},
		{ServiceTypeEducation, "education_assistance"},
		{ServiceTypeTransport, "transport_assistance"},
		{ServiceTypeGeneral, "general_assistance"},
	}

	for _, tc := range tests {
		t.Run(string(tc.serviceType), func(t *testing.T) {
			assert.Equal(t, tc.expected, WorkflowKeyForServiceType(tc.serviceType))
		})
	}
}

func TestWorkflowKeyForServiceType_Default(t *testing.T) {
	// Unknown service types must fall back to the generic workflow so that
	// every case is bound to an authoritative workflow definition.
	assert.Equal(t, "general_assistance", WorkflowKeyForServiceType("UNKNOWN_TYPE"))
	assert.Equal(t, "general_assistance", WorkflowKeyForServiceType(""))
}

func TestWorkflowKeyForServiceType_CoversAllServiceTypes(t *testing.T) {
	// Every declared ServiceType constant must resolve to a non-empty key.
	for _, st := range []ServiceType{
		ServiceTypeGeneral, ServiceTypeEmergency, ServiceTypeFinancial,
		ServiceTypeFood, ServiceTypeShelter, ServiceTypeMedical,
		ServiceTypeEducation, ServiceTypeTransport,
	} {
		assert.NotEmpty(t, WorkflowKeyForServiceType(st), "service type %s must map to a workflow key", st)
	}
}
