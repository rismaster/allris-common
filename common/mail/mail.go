package mail

import "time"

type SomethingNewMessage struct {
	EntityType string
	Name       string
	ParentKind string
	ParentName string
	Message    string
	Time       time.Time
}
