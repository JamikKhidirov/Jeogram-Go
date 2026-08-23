package handler

import (
	"testing"

	"github.com/jeogram/messenger/internal/modules/message/domain"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

func TestQuoteMessageValidation(t *testing.T) {
	tests := []struct {
		name  string
		req   domain.QuoteRequest
		wantErr bool
	}{
		{"valid", domain.QuoteRequest{Text: "hello"}, false},
		{"empty text", domain.QuoteRequest{Text: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validator.ValidateStruct(&tt.req)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("got error %v, want %v", hasErr, tt.wantErr)
			}
		})
	}
}
