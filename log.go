package main

import (
	"bytes"
	"fmt"
	"github.com/RangelReale/appstatsd/data"
	"github.com/RangelReale/gostatsd/statsd"
	"io"
	"net"
	"strconv"
	"time"
)

// Receive log messages via udp
func ServerLog() {
	c, err := net.ListenPacket("udp", fmt.Sprintf("%s:%d", Configuration.ListenHost, Configuration.LogPort))
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

		// don't require line to end in \n, if EOF use received line if valid
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
			ldata, err := serverLogParseLine(line)
			if err != nil {
				log.Error("error parsing line %q from %s: %s", line, addr, err)
				continue
			}

			// send message to database
			DatabaseChan <- DBMessage{log: ldata}

			if Configuration.ErrorStatistics {
				// log errors in "error"
				if ldata.Level < data.WARNING {
					DatabaseChan <- DBMessage{metrics: &statsd.Metric{
						Type:   statsd.COUNTER,
						Bucket: fmt.Sprintf("%s.error.ct", ldata.App),
						Value:  1,
					}}
				} else if ldata.Level == data.WARNING {
					DatabaseChan <- DBMessage{metrics: &statsd.Metric{
						Type:   statsd.COUNTER,
						Bucket: fmt.Sprintf("%s.error.wct", ldata.App),
						Value:  1,
					}}
				}
			}
		}

		if readerr != nil && readerr == io.EOF {
			// if was EOF, finished handling
			return
		}
	}
}

// APP:LEVEL:MESSAGEID:MESSAGE
func serverLogParseLine(line []byte) (*data.LogData, error) {
	ldata := &data.LogData{Date: time.Now()}

	buf := bytes.NewBuffer(line)

	app, err := buf.ReadBytes(':')
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	ldata.App = string(app[:len(app)-1])

	level, err := buf.ReadBytes(':')
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	levelint, err := strconv.ParseInt(string(level[:len(level)-1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	ldata.Level = data.LogLevel(levelint)

	messageid, err := buf.ReadBytes(':')
	if err != nil {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	ldata.MessageId = string(messageid[:len(messageid)-1])

	message := buf.Bytes()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("error parsing log: %s", err)
	}
	ldata.Message = string(message[:len(message)])

	return ldata, nil
}
