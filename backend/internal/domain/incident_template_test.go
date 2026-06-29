package domain

import (
	"errors"
	"testing"
)

func TestIncidentTemplateValidate(t *testing.T) {
	tests := []struct {
		name string
		tmpl IncidentTemplate
		want error
	}{
		{
			name: "valid",
			tmpl: IncidentTemplate{Name: "DB degradation", DefaultImpact: ImpactMajor},
			want: nil,
		},
		{
			name: "valid with empty templates and none impact",
			tmpl: IncidentTemplate{Name: "Blank", DefaultImpact: ImpactNone},
			want: nil,
		},
		{
			name: "empty name",
			tmpl: IncidentTemplate{Name: "", DefaultImpact: ImpactMinor},
			want: ErrInvalidTemplateName,
		},
		{
			name: "invalid impact",
			tmpl: IncidentTemplate{Name: "x", DefaultImpact: IncidentImpact("bogus")},
			want: ErrInvalidIncidentImpact,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tmpl.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}
