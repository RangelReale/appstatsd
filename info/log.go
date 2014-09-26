package info

import (
	"github.com/RangelReale/appstatsd/data"
	"gopkg.in/mgo.v2"
)

type LogQuery struct {
	Amount int
	App    string
}

func QueryLog(db *mgo.Database, logquery *LogQuery) ([]*data.LogData, error) {
	amount := logquery.Amount
	if amount < 0 {
		amount = 100
	}

	// log
	c_log := db.C("log")
	c_log.EnsureIndex(mgo.Index{
		Key:        []string{"-dt"},
		Background: true,
		Sparse:     true,
	})

	fdata := make([]*data.LogData, 0)

	query := c_log.Find(nil).Sort("-dt").Limit(amount).Iter()
	var flog *data.LogData

	for query.Next(&flog) {
		fdata = append(fdata, flog)
	}

	if err := query.Close(); err != nil {
		return nil, err
	}

	return fdata, nil
}
