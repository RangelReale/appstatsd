package data

import (
	"time"
)

type LogLevel int

const (
	CRITICAL LogLevel = 1
	ERROR             = 2
	WARNING           = 3
	NOTICE            = 4
	INFO              = 5
	DEBUG             = 6
)

type LogData struct {
	Date      time.Time `json:"date" bson:"dt"`
	Level     LogLevel  `json:"level" bson:"lv"`
	App       string    `json:"app" bson:"app,omitempty"`
	MessageId string    `json:"mid,omitempty" bson:"mid,omitempty"`
	Message   string    `json:"msg" bson:"m"`
}
