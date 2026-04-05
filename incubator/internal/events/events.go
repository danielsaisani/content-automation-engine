package events

import "time"

type Event struct {
	ID        string
	Timestamp time.Time
	Data      interface{}
}

type 