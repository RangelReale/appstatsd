package main

import (
	"fmt"
	"github.com/RangelReale/gostatsd/statsd"
)

func ServerStatsd() {
	f := func(m statsd.Metric) {
		//log.Debug("Metric received: %s: %s [%f]", m.Type.String(), m.Bucket, m.Value)
		DatabaseChan <- DBMessage{metrics: &m}
	}
	r := statsd.MetricReceiver{fmt.Sprintf("%s:%d", Configuration.ListenHost, Configuration.StatsdPort), statsd.HandlerFunc(f)}
	r.ListenAndReceive()
}
