package main

import (
	"bytes"
	"fmt"
	"github.com/RangelReale/gostatsd/statsd"
	"io"
	"net"
	"strconv"
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
	Date      time.Time `bson:"dt"`
	Level     LogLevel  `bson:"lv"`
	App       string    `bson:"app,omitempty"`
	MessageId string    `bson:"mid,omitempty"`
	Message   string    `bson:"m"`
}

func ServerLog() {
	c, err := net.ListenPacket("udp", fmt.Sprintf(":%d", Configuration.LogPort))
	if err != nil {
		log.Fatal("Error creating log server: %s", err.Error())
	}

	defer c.Close()

	msg := make([]byte, 1024)
	for {
		nbytes, addr, err := c.ReadFrom(msg)
		if err != nil {
			log.Error("%s", err)
			continue
		}
		buf := make([]byte, nbytes)
		copy(buf, msg[:nbytes])
		go serverLogHandleMessage(addr, buf)
	}
	panic("not reached")
}

func serverLogHandleMessage(addr net.Addr, msg []byte) {
	buf := bytes.NewBuffer(msg)
	for {
		line, readerr := buf.ReadBytes('\n')

		// protocol does not require line to end in \n, if EOF use received line if valid
		if readerr != nil && readerr != io.EOF {
			log.Error("error reading message from %s: %s", addr, readerr)
			return
		} else if readerr != io.EOF {
			// remove newline, only if not EOF
			if len(line) > 0 {
				line = line[:len(line)-1]
			}
		}

		// Only process lines with more than one character
		if len(line) > 1 {
			data, err := serverLogParseLine(line)
			if err != nil {
				log.Error("error parsing line %q from %s: %s", line, addr, err)
				continue
			}
			DatabaseChan <- DBMessage{log: data}
			// log errors in "error"
			if data.Level < WARNING {
				DatabaseChan <- DBMessage{metrics: &statsd.Metric{
					Type:   statsd.COUNTER,
					Bucket: fmt.Sprintf("%s.error.ct", data.App),
					Value:  1,
				}}
			}
		}

		if readerr != nil && readerr == io.EOF {
			// if was EOF, finished handling
			return
		}
	}
}

// APP:LEVEL:MESSAGEID:MESSAGE
func serverLogParseLine(line []byte) (*LogData, error) {
	data := &LogData{Date: time.Now()}

	buf := bytes.NewBuffer(line)

	app, err := buf.ReadBytes(':')
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	data.App = string(app[:len(app)-1])

	level, err := buf.ReadBytes(':')
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	levelint, err := strconv.ParseInt(string(level[:len(level)-1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	data.Level = LogLevel(levelint)

	messageid, err := buf.ReadBytes(':')
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	data.MessageId = string(messageid[:len(messageid)-1])

	message := buf.Bytes()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	data.Message = string(message[:len(message)])

	return data, nil
}
