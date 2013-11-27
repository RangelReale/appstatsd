package main

import (
	"fmt"
	"github.com/RangelReale/gostatsd/statsd"
	"time"
)

func ServerStatsd() {
	fagg := func(m statsd.MetricMap) {
		log.Debug("%s", m)
	}

	aggregator := statsd.NewMetricAggregator(statsd.MetricSenderFunc(fagg), time.Duration(Configuration.FlushInterval)*time.Second)
	go aggregator.Aggregate()

	f := func(m statsd.Metric) {
		log.Debug("%s", m)
		aggregator.MetricChan <- m
	}
	r := statsd.MetricReceiver{fmt.Sprintf(":%d", Configuration.StatsdPort), statsd.HandlerFunc(f)}
	r.ListenAndReceive()
}
