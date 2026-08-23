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
//	@Summary	Upload media (image or voice)
//	@Tags		media
//	@Accept		multipart/form-data
//	@Produce	json
//	@Param		type	formData	string	true	"image or voice"
//	@Param		file	formData	file	true	"file to upload"
//	@Success	201	{object}	response.APIResponse
//	@Router		/media/upload [post]
//	@Security	BearerAuth
func (h *MediaHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.JWTAuth(h.jwt)).Post("/media/upload", h.Upload)
	r.Get("/media/{type}/{file}", h.Serve)
}

// Upload загружает медиафайл (изображение или голос) и возвращает URL.
// @Summary Загрузить медиа
// @Tags media
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param type formData string true "image or voice"
// @Param file formData file true "файл"
// @Success 201 {object} response.APIResponse
// @Router /media/upload [post]
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "could not parse multipart form")
		return
	}
	mediaType := domain.MediaType(r.FormValue("type"))
	if mediaType != domain.MediaImage && mediaType != domain.MediaVoice {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "type must be 'image' or 'voice'")
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
	url, err := h.svc.Save(mediaType, header.Header.Get("Content-Type"), header.Filename, data)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	response.WriteCreated(w, map[string]string{"url": url, "type": string(mediaType)})
}

// Serve отдаёт загруженный медиафайл по URL.
// @Summary Отдать медиафайл
// @Tags media
// @Produce application/octet-stream
// @Param type path string true "image or voice"
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
