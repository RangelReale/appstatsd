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

// message to send on DatabaseChan to notify of new data
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

// handle metrics received from statsd
// bucket name must be in this format:
// appname.info1#param1#param2.info2#param1.infoX.field
// data is saved for day, hour, and 15 minute intervals
func dbHandleMetrics(m *statsd.Metric) {
	values := strings.Split(m.Bucket, ".")
	if len(values) < 3 {
		log.Error("Invalid bucket name - at least 3 items dot-separated items are required: %s", m.Bucket)
		return
	}
	if strings.HasPrefix(m.Bucket, "_") {
		log.Error("Invalid bucket name - cannot start with underline: %s", m.Bucket)
		return
	}

	// first item - app
	app := values[0]

	// last item - field name
	name := values[len(values)-1]

	// remove first and last item
	values = values[1 : len(values)-1]

	tm := time.Now().UTC()
	// 15 minute aggregation
	minute := int(tm.Minute()/15.0) * 15

	var idata *bson.M

	switch m.Type {
	case statsd.COUNTER:
		idata = &bson.M{
			"$inc": bson.M{
				fmt.Sprintf("_dy.c_%s", name):                                 m.Value,
				fmt.Sprintf("_hr.h_%d.c_%s", tm.Hour(), name):                 m.Value,
				fmt.Sprintf("_hr.h_%d.mn.m_%d.c_%s", tm.Hour(), minute, name): m.Value,
			},
		}
	case statsd.TIMER:
		idata = &bson.M{
			"$inc": bson.M{
				fmt.Sprintf("_dy.t_%s", name):                                  m.Value,
				fmt.Sprintf("_dy.tc_%s", name):                                 1,
				fmt.Sprintf("_hr.h_%d.t_%s", tm.Hour(), name):                  m.Value,
				fmt.Sprintf("_hr.h_%d.tc_%s", tm.Hour(), name):                 1,
				fmt.Sprintf("_hr.h_%d.mn.m_%d.t_%s", tm.Hour(), minute, name):  m.Value,
				fmt.Sprintf("_hr.h_%d.mn.m_%d.tc_%s", tm.Hour(), minute, name): 1,
			},
		}
	case statsd.GAUGE:
		idata = &bson.M{
			"$inc": bson.M{
				fmt.Sprintf("_dy.g_%s", name):                                  m.Value,
				fmt.Sprintf("_dy.gc_%s", name):                                 1,
				fmt.Sprintf("_hr.h_%d.g_%s", tm.Hour(), name):                  m.Value,
				fmt.Sprintf("_hr.h_%d.gc_%s", tm.Hour(), name):                 1,
				fmt.Sprintf("_hr.h_%d.mn.m_%d.g_%s", tm.Hour(), minute, name):  m.Value,
				fmt.Sprintf("_hr.h_%d.mn.m_%d.gc_%s", tm.Hour(), minute, name): 1,
			},
		}
	}

	if idata != nil {
		baseq := bson.M{
			"_dt": tm.Format("2006-01-02"),
		}
		baseqapp := bson.M{
			"_dt":  tm.Format("2006-01-02"),
			"_app": app,
		}

		// all collections start with _a
		c_base := "_a"

		// loop on info. Each can have parameters separated by #
		for _, iv := range values {
			info := strings.Split(iv, "#")
			if strings.HasPrefix(info[0], "_") {
				log.Error("Invalid bucket name - info cannot start with underline: %s", info[0])
				return
			}

			c_base = c_base + "_" + info[0]
			if len(c_base) == 0 {
				log.Error("Invalid bucket name - info cannot be blank: %s", m.Bucket)
				return
			}

			// separate database for total and per-app
			c := dbdb.C(c_base)
			capp := dbdb.C(fmt.Sprintf("%s_app", c_base))

			// loop parameters
			for ridx, rv := range info[1:] {
				if rv != "" {
					var pname string
					// from second parameter on, add index to parameter name, starting from 1
					if ridx > 0 {
						pname = fmt.Sprintf("%s%d", info[0], ridx)
					} else {
						pname = info[0]
					}

					if strings.HasPrefix(pname, "_") {
						log.Error("Invalid bucket name - parameter cannot start with underline: %s", pname)
						return
					}

					// add parameter to queries
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

func DBConnectClone() (*mgo.Session, error) {
	if dbsession == nil {
		err := dbConnect()
		if err != nil {
			return nil, err
		}
	}

	return dbsession.Clone(), nil
}
