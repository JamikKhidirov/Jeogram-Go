package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/e2ee/domain"
	"github.com/jeogram/messenger/internal/modules/e2ee/repository"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
	"github.com/jeogram/messenger/internal/pkg/validator"
)

// E2EEHandler exposes prekey-bundle management for end-to-end encryption.
type E2EEHandler struct {
	repo *repository.PreKeyRepository
	jwt  *auth.JWT
}

func NewE2EEHandler(repo *repository.PreKeyRepository, jwt *auth.JWT) *E2EEHandler {
	return &E2EEHandler{repo: repo, jwt: jwt}
}

// RegisterRoutes mounts E2EE endpoints.
//
//	@Summary	Upload prekey bundle
//	@Tags		e2ee
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body	domain.PreKeyUpload	true	"prekey bundle"
//	@Success	200	{object}	response.APIResponse
//	@Router		/e2ee/prekeys [put]
//	@Security	BearerAuth
func (h *E2EEHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Put("/e2ee/prekeys", h.Upload)
	r.With(middleware.JWTAuth(h.jwt)).Get("/e2ee/prekeys/{user_id}", h.Fetch)
}

// Upload stores (replacing) the caller's one-time prekeys.
func (h *E2EEHandler) Upload(w http.ResponseWriter, req *http.Request) {
	userID := middleware.UserID(req)
	var body domain.PreKeyUpload
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if errs := validator.ValidateStruct(&body); len(errs) > 0 {
		response.WriteError(w, http.StatusUnprocessableEntity, "validation_error", "prekeys required")
		return
	}
	if err := h.repo.Upsert(req.Context(), userID, body.PreKeys); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]interface{}{"status": "uploaded", "count": len(body.PreKeys)})
}

// Fetch claims a single unused prekey for the requested user (one-time).
//
//	@Summary	Get a one-time prekey for a user
//	@Tags		e2ee
//	@Produce	json
//	@Security	BearerAuth
//	@Param		user_id	path	string	true	"user id"
//	@Success	200	{object}	response.APIResponse
//	@Router		/e2ee/prekeys/{user_id} [get]
//	@Security	BearerAuth
func (h *E2EEHandler) Fetch(w http.ResponseWriter, req *http.Request) {
	target := chi.URLParam(req, "user_id")
	pk, err := h.repo.ClaimOne(req.Context(), target)
	if err != nil {
		if errors.Is(err, repository.ErrNoPreKey) {
			response.WriteError(w, http.StatusNotFound, "not_found", "no prekey available")
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]interface{}{
		"user_id":       pk.UserID,
		"key_id":        pk.KeyID,
		"public_key":    pk.PublicKey,
		"signature_key": pk.SignatureKey,
	})
}
