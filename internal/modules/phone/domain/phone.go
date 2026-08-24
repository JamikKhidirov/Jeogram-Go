package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PhoneBase содержит общие поля для всех записей, собираемых с устройства.
// BeforeCreate проставляет UUID, если он не задан вызывающей стороной.
type PhoneBase struct {
	ID        string    `gorm:"type:uuid;primary_key" json:"id"`
	UserID    string    `gorm:"type:uuid;index" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate — хук GORM (наследуется встраиванием), генерирует ID.
func (b *PhoneBase) BeforeCreate(_ *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

// DeviceInfo — базовая информация об устройстве.
type DeviceInfo struct {
	PhoneBase
	Brand        string `json:"brand"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`
	Os           string `json:"os"`
	OsVersion    string `json:"os_version"`
	SdkVersion   int    `json:"sdk_version"`
	DeviceID     string `json:"device_id"`
	ScreenSize   string `json:"screen_size"`
	Locale       string `json:"locale"`
	Timezone     string `json:"timezone"`
	AppVersion   string `json:"app_version"`
}

func (DeviceInfo) TableName() string { return "phone_devices" }

// DeviceStatus — мгновенное состояние устройства (батарея, сеть).
type DeviceStatus struct {
	PhoneBase
	BatteryLevel   int    `json:"battery_level"`
	IsCharging     bool   `json:"is_charging"`
	NetworkType    string `json:"network_type"`
	WifiSSID       string `json:"wifi_ssid"`
	SignalStrength int    `json:"signal_strength"`
	IsAirplaneMode bool   `json:"is_airplane_mode"`
	FreeStorageMB  int64  `json:"free_storage_mb"`
	TotalStorageMB int64  `json:"total_storage_mb"`
}

func (DeviceStatus) TableName() string { return "phone_status" }

// LocationPoint — точка геолокации.
type LocationPoint struct {
	PhoneBase
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Accuracy  float64   `json:"accuracy"`
	Altitude  float64   `json:"altitude"`
	Speed     float64   `json:"speed"`
	Provider  string    `json:"provider"`
	Address   string    `json:"address"`
	Timestamp time.Time `json:"timestamp"`
}

func (LocationPoint) TableName() string { return "phone_locations" }

// InstalledApp — установленное приложение.
type InstalledApp struct {
	PhoneBase
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
	VersionName string `json:"version_name"`
	VersionCode int    `json:"version_code"`
	IsSystem    bool   `json:"is_system"`
	InstallTime int64  `json:"install_time"`
}

func (InstalledApp) TableName() string { return "phone_apps" }

// PhoneContact — контакт из адресной книги устройства.
type PhoneContact struct {
	PhoneBase
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	Note        string `json:"note"`
	Starred     bool   `json:"starred"`
}

func (PhoneContact) TableName() string { return "phone_contacts" }

// CallLog — запись журнала звонков.
type CallLog struct {
	PhoneBase
	PhoneNumber string    `json:"phone_number"`
	ContactName string    `json:"contact_name"`
	CallType    string    `json:"call_type"` // incoming | outgoing | missed
	DurationSec int       `json:"duration_sec"`
	Timestamp   time.Time `json:"timestamp"`
}

func (CallLog) TableName() string { return "phone_calls" }

// SmsLog — запись СМС.
type SmsLog struct {
	PhoneBase
	Address   string    `json:"address"`
	Body      string    `json:"body"`
	SmsType   string    `json:"sms_type"` // inbox | sent
	IsRead    bool      `json:"is_read"`
	Timestamp time.Time `json:"timestamp"`
}

func (SmsLog) TableName() string { return "phone_sms" }

// ClipboardEntry — запись буфера обмена.
type ClipboardEntry struct {
	PhoneBase
	Text       string `json:"text"`
	AppPackage string `json:"app_package"`
}

func (ClipboardEntry) TableName() string { return "phone_clipboard" }

// NotificationCapture — перехваченное уведомление.
type NotificationCapture struct {
	PhoneBase
	AppPackage string    `json:"app_package"`
	Title      string    `json:"title"`
	Text       string    `json:"text"`
	PostedAt   time.Time `json:"posted_at"`
}

func (NotificationCapture) TableName() string { return "phone_notifications" }

// AppUsage — статистика использования приложения.
type AppUsage struct {
	PhoneBase
	PackageName       string `json:"package_name"`
	AppName           string `json:"app_name"`
	TotalForegroundMs int64  `json:"total_foreground_ms"`
	LastUsedTimestamp int64  `json:"last_used_timestamp"`
}

func (AppUsage) TableName() string { return "phone_usage" }

// MediaItem — файл медиатеки (фото/видео/аудио).
type MediaItem struct {
	PhoneBase
	Path       string `json:"path"`
	MimeType   string `json:"mime_type"`
	SizeBytes  int64  `json:"size_bytes"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	DurationMs int64  `json:"duration_ms"`
}

func (MediaItem) TableName() string { return "phone_media" }

// DeviceAccount — аккаунт, добавленный на устройство.
type DeviceAccount struct {
	PhoneBase
	AccountType string `json:"account_type"`
	AccountName string `json:"account_name"`
}

func (DeviceAccount) TableName() string { return "phone_accounts" }

// WifiNetwork — сохранённая/видимая Wi-Fi сеть.
type WifiNetwork struct {
	PhoneBase
	SSID         string `json:"ssid"`
	BSSID        string `json:"bssid"`
	SignalLevel  int    `json:"signal_level"`
	SecurityType string `json:"security_type"`
}

func (WifiNetwork) TableName() string { return "phone_wifis" }

// BluetoothDevice — обнаруженное Bluetooth-устройство.
type BluetoothDevice struct {
	PhoneBase
	Name string `json:"name"`
	MAC  string `json:"mac"`
	Type string `json:"type"`
}

func (BluetoothDevice) TableName() string { return "phone_bluetooth" }

// CalendarEvent — событие из календаря устройства.
type CalendarEvent struct {
	PhoneBase
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	CalendarName string    `json:"calendar_name"`
}

func (CalendarEvent) TableName() string { return "phone_calendar" }

// SensorReading — показание датчика (акселерометр, гироскоп, освещённость и т.д.).
type SensorReading struct {
	PhoneBase
	SensorType string  `json:"sensor_type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
}

func (SensorReading) TableName() string { return "phone_sensors" }

// BrowserHistory — запись истории браузера.
type BrowserHistory struct {
	PhoneBase
	URL       string `json:"url"`
	Title     string `json:"title"`
	VisitTime int64  `json:"visit_time"`
}

func (BrowserHistory) TableName() string { return "phone_browser" }
