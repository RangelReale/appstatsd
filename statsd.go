package main

import (
	"fmt"
	"github.com/RangelReale/gostatsd/statsd"
)

func ServerStatsd() {
	f := func(m statsd.Metric) {
		DatabaseChan <- DBMessage{metrics: &m}
	}
	r := statsd.MetricReceiver{fmt.Sprintf("%s:%d", Configuration.ListenHost, Configuration.StatsdPort), statsd.HandlerFunc(f)}
	r.ListenAndReceive()
}
