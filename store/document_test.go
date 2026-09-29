package store

import "testing"

func TestValidateDocumentStatusTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    DocumentStatus
		to      DocumentStatus
		wantErr bool
	}{
		{name: "queue upload", from: DocumentStatusUploaded, to: DocumentStatusQueued},
		{name: "start parsing", from: DocumentStatusQueued, to: DocumentStatusParsing},
		{name: "finish parsing", from: DocumentStatusParsing, to: DocumentStatusReady},
		{name: "fail parsing", from: DocumentStatusParsing, to: DocumentStatusFailed},
		{name: "retry failure", from: DocumentStatusFailed, to: DocumentStatusQueued},
		{name: "reparse ready document", from: DocumentStatusReady, to: DocumentStatusQueued},
		{name: "enable a new parser", from: DocumentStatusUnsupported, to: DocumentStatusQueued},
		{name: "idempotent update", from: DocumentStatusParsing, to: DocumentStatusParsing},
		{name: "cannot skip queue", from: DocumentStatusUploaded, to: DocumentStatusParsing, wantErr: true},
		{name: "cannot rewrite ready as parsing", from: DocumentStatusReady, to: DocumentStatusParsing, wantErr: true},
		{name: "cannot restore upload state", from: DocumentStatusFailed, to: DocumentStatusUploaded, wantErr: true},
		{name: "invalid current state", from: DocumentStatus("INVALID"), to: DocumentStatusQueued, wantErr: true},
		{name: "invalid target state", from: DocumentStatusQueued, to: DocumentStatus("INVALID"), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDocumentStatusTransition(test.from, test.to)
			if test.wantErr && err == nil {
				t.Fatalf("ValidateDocumentStatusTransition(%q, %q) returned nil", test.from, test.to)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidateDocumentStatusTransition(%q, %q) returned error: %v", test.from, test.to, err)
			}
		})
	}
}

func TestDocumentStatusAndParseResultValidity(t *testing.T) {
	for _, status := range []DocumentStatus{
		DocumentStatusUploaded,
		DocumentStatusQueued,
		DocumentStatusParsing,
		DocumentStatusReady,
		DocumentStatusFailed,
		DocumentStatusUnsupported,
	} {
		if !status.IsValid() {
			t.Errorf("expected status %q to be valid", status)
		}
	}

	for _, result := range []DocumentParseResult{
		DocumentParseResultRunning,
		DocumentParseResultSucceeded,
		DocumentParseResultFailed,
		DocumentParseResultUnsupported,
	} {
		if !result.IsValid() {
			t.Errorf("expected parse result %q to be valid", result)
		}
	}
}
