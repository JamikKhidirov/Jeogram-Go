package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/media/domain"
	"github.com/jeogram/messenger/internal/modules/media/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
)

// MediaHandler handles uploads and static serving of media.
type MediaHandler struct {
	svc *service.MediaService
	jwt *auth.JWT
}

func NewMediaHandler(svc *service.MediaService, jwt *auth.JWT) *MediaHandler {
	return &MediaHandler{svc: svc, jwt: jwt}
}

// RegisterRoutes mounts media endpoints.
//
//	@Summary	Upload media (image, voice, video or document)
//	@Tags		media
//	@Accept		multipart/form-data
//	@Produce	json
//	@Param		type	formData	string	true	"image|voice|video|document"
//	@Param		file	formData	file	true	"file to upload"
//	@Success	201	{object}	response.APIResponse
//	@Router		/media/upload [post]
//	@Security	BearerAuth
func (h *MediaHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/media/upload", h.Upload)
	r.Get("/media/{type}/{file}", h.Serve)
	r.Get("/media/{id}/download", h.Download)
	r.Get("/media/{id}/thumbnail", h.Thumbnail)
	r.With(middleware.JWTAuth(h.jwt)).Delete("/media/{id}", h.DeleteMedia)
}

// Upload загружает медиафайл и возвращает его id + URL.
// @Summary Загрузить медиа
// @Tags media
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param type formData string true "image|voice|video|document"
// @Param file formData file true "файл"
// @Success 201 {object} response.APIResponse
// @Router /media/upload [post]
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "could not parse multipart form")
		return
	}
	mediaType := domain.MediaType(r.FormValue("type"))
	switch mediaType {
	case domain.MediaImage, domain.MediaVoice, domain.MediaVideo, domain.MediaDocument:
		// ok
	default:
		response.WriteError(w, http.StatusBadRequest, "bad_request", "type must be one of image|voice|video|document")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", "could not read file")
		return
	}
	owner := middleware.UserID(r)
	rec, err := h.svc.Save(r.Context(), owner, mediaType, header.Header.Get("Content-Type"), header.Filename, data)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteCreated(w, map[string]interface{}{"id": rec.ID, "url": rec.URL, "type": string(rec.Type)})
}

// Serve отдаёт загруженный медиафайл по URL.
// @Summary Отдать медиафайл
// @Tags media
// @Produce application/octet-stream
// @Param type path string true "image|voice|video|document"
// @Param file path string true "имя файла"
// @Success 200
// @Router /media/{type}/{file} [get]
func (h *MediaHandler) Serve(w http.ResponseWriter, r *http.Request) {
	mediaType := chi.URLParam(r, "type")
	file := chi.URLParam(r, "file")
	url := strings.TrimRight(h.svc.BaseURL(), "/") + "/" + mediaType + "/" + file
	path, err := h.svc.PathFor(url)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	http.ServeFile(w, r, path)
}

// Download отдаёт файл по id как вложение (Content-Disposition: attachment).
// @Summary Скачать файл по id
// @Tags media
// @Produce application/octet-stream
// @Param id path string true "id медиафайла"
// @Success 200
// @Router /media/{id}/download [get]
func (h *MediaHandler) Download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	path, err := h.svc.PathFor(rec.URL)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+rec.Filename+"\"")
	http.ServeFile(w, r, path)
}

// Thumbnail отдаёт превью (для image/video — сам файл; для документов — 404).
// @Summary Получить превью/миниатюру
// @Tags media
// @Produce application/octet-stream
// @Param id path string true "id медиафайла"
// @Success 200
// @Router /media/{id}/thumbnail [get]
func (h *MediaHandler) Thumbnail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	if rec.Type != domain.MediaImage && rec.Type != domain.MediaVideo {
		response.WriteError(w, http.StatusNotFound, "not_found", "no thumbnail for this media type")
		return
	}
	path, err := h.svc.PathFor(rec.URL)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	http.ServeFile(w, r, path)
}

// DeleteMedia удаляет файл и его метаданные (только владелец или админ).
// @Summary Удалить медиафайл
// @Tags media
// @Produce json
// @Security BearerAuth
// @Param id path string true "id медиафайла"
// @Success 200 {object} response.APIResponse
// @Router /media/{id} [delete]
func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserID(r)
	rec, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	if rec.OwnerID != userID {
		response.WriteError(w, http.StatusForbidden, "forbidden", "not owner")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "deleted"})
}
