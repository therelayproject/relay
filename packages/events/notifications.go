package events

// PushTarget identifies the delivery channel for a push notification.
type PushTarget string

const (
	PushTargetFCM     PushTarget = "fcm"     // Android + web
	PushTargetAPNS    PushTarget = "apns"    // iOS + macOS
	PushTargetWebPush PushTarget = "webpush" // browsers
)

// NotificationPushEvent is published by the Notification Service after it
// determines which users need a push for a given MessageCreatedEvent.
// Consumed by: Notification Service's own push-sender goroutines (internal fan-out).
type NotificationPushEvent struct {
	UserID    int64      `json:"user_id"`
	Target    PushTarget `json:"target"`
	DeviceToken string   `json:"device_token"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	// Data is passed through to the push payload for deep-linking.
	Data map[string]string `json:"data,omitempty"`
}
