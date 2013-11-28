package main

import (
	"fmt"
	"github.com/RangelReale/gostatsd/statsd"
)

func ServerStatsd() {
	f := func(m statsd.Metric) {
		//log.Debug("%s", m)
		DatabaseChan <- DBMessage{metrics: &m}
	}
	r := statsd.MetricReceiver{fmt.Sprintf(":%d", Configuration.StatsdPort), statsd.HandlerFunc(f)}
	r.ListenAndReceive()
}
