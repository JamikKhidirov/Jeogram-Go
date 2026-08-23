package validator

import (
	"testing"
)

type sample struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=3"`
}

func TestValidateStruct(t *testing.T) {
	valid := sample{Email: "a@b.com", Name: "john"}
	if errs := ValidateStruct(&valid); len(errs) != 0 {
		t.Fatalf("ожидалось отсутствие ошибок, получено: %v", errs)
	}

	invalid := sample{Email: "not-an-email", Name: "jo"}
	errs := ValidateStruct(&invalid)
	if len(errs) == 0 {
		t.Fatal("ожидались ошибки валидации")
	}
	if _, ok := errs["email"]; !ok {
		t.Fatalf("ожидалась ошибка для поля email, получено: %v", errs)
	}
	if _, ok := errs["name"]; !ok {
		t.Fatalf("ожидалась ошибка для поля name, получено: %v", errs)
	}
}
