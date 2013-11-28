package main

import (
	"fmt"
	"github.com/RangelReale/gostatsd/statsd"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"strings"
	"time"
)

var (
	dbsession    *mgo.Session
	dbdb         *mgo.Database
	dblogc       *mgo.Collection
	DatabaseChan chan DBMessage
)

type DBMessage struct {
	metrics *statsd.Metric
	log     *LogData
}

func init() {
	DatabaseChan = make(chan DBMessage, 1000)
}

func ServerDatabase() {
	dbConnect()

	for proc := range DatabaseChan {
		if err := dbConnect(); err != nil {
			log.Error("Could not connect to database: %s", err)
			continue
		}

		if proc.metrics != nil {
			dbHandleMetrics(proc.metrics)
		}

		if proc.log != nil {
			dblogc.Insert(proc.log)
		}
	}
}

func dbHandleMetrics(m *statsd.Metric) {
	values := strings.Split(m.Bucket, ".")
	if len(values) < 3 {
		log.Error("Invalid bucket name: %s", m.Bucket)
		return
	}
	if strings.HasPrefix(m.Bucket, "_") {
		log.Error("Invalid bucket name: %s", m.Bucket)
		return
	}

	app := values[0]
	name := values[len(values)-1]
	values = values[1 : len(values)-1]
	tm := time.Now().UTC()
	// 15 minute aggregation
	minute := int(tm.Minute()/15.0) * 15

	var idata *bson.M

	switch m.Type {
	case statsd.COUNTER:
		idata = &bson.M{
			"$inc": bson.M{
				fmt.Sprintf("dy.c_%s", name):                                 m.Value,
				fmt.Sprintf("hr.h_%d.c_%s", tm.Hour(), name):                 m.Value,
				fmt.Sprintf("hr.h_%d.mn.m_%d.c_%s", tm.Hour(), minute, name): m.Value,
			},
		}
	case statsd.TIMER:
		idata = &bson.M{
			"$inc": bson.M{
				fmt.Sprintf("dy.t_%s", name):                                  m.Value,
				fmt.Sprintf("dy.tc_%s", name):                                 1,
				fmt.Sprintf("hr.h_%d.t_%s", tm.Hour(), name):                  m.Value,
				fmt.Sprintf("hr.h_%d.tc_%s", tm.Hour(), name):                 1,
				fmt.Sprintf("hr.h_%d.mn.m_%d.t_%s", tm.Hour(), minute, name):  m.Value,
				fmt.Sprintf("hr.h_%d.mn.m_%d.tc_%s", tm.Hour(), minute, name): 1,
			},
		}
	case statsd.GAUGE:
	}

	if idata != nil {
		baseq := bson.M{
			"dt": tm.Format("2006-01-02"),
		}
		baseqapp := bson.M{
			"dt":  tm.Format("2006-01-02"),
			"app": app,
		}

		c_base := "_a"
		for _, iv := range values {
			info := strings.Split(iv, "#")

			c_base = c_base + "_" + info[0]

			if len(c_base) == 0 {
				log.Error("Invalid bucket name: %s", m.Bucket)
				return
			}

			c := dbdb.C(c_base)
			capp := dbdb.C(fmt.Sprintf("%s_app", c_base))

			for ridx, rv := range info[1:] {
				if rv != "" {
					var pname string
					if ridx > 0 {
						pname = fmt.Sprintf("%s%d", info[0], ridx)
					} else {
						pname = info[0]
					}

					baseq[pname] = rv
					baseqapp[pname] = rv
				}
			}

			// general
			_, err := c.Upsert(baseq, *idata)
			if err != nil {
				log.Error("Error saving log record: %s", err)
			}

			// by app
			if app != "" {
				_, err = capp.Upsert(baseqapp, *idata)
				if err != nil {
					log.Error("Error saving connection app record: %s", err)
				}
			}
		}
	}
}

func dbConnect() error {
	if dbsession != nil {
		return nil
	}

	log.Debug("Connecting to database")

	var mgourl string
	if Configuration.MGOUsername != "" {
		mgourl = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s",
			Configuration.MGOUsername, Configuration.MGOPassword,
			Configuration.MGOHost, Configuration.MGOPort,
			Configuration.MGODBName)
	} else {
		mgourl = Configuration.MGOHost
	}

	var err error
	dbsession, err = mgo.Dial(mgourl)
	if err == nil {
		dbdb = dbsession.DB(Configuration.MGODBName)
		dblogc = dbdb.C("log")
	} else {
		dbsession = nil
	}
	return err
}
