package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteOKEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteOK(rec, map[string]string{"hello": "world"})

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", rec.Code)
	}
	var body APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("некорректный JSON: %v", err)
	}
	if !body.Success {
		t.Fatal("success должен быть true")
	}
	if body.Data == nil {
		t.Fatal("data не должна быть пустой")
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "bad_request", "invalid")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидался статус 400, получен %d", rec.Code)
	}
	var body APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("некорректный JSON: %v", err)
	}
	if body.Success {
		t.Fatal("success должен быть false")
	}
	if body.Error == nil || body.Error.Code != "bad_request" {
		t.Fatalf("неверный error: %+v", body.Error)
	}
}
