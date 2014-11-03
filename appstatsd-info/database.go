package main

import (
	"fmt"
	"gopkg.in/mgo.v2"
)

var (
	dbsession *mgo.Session
	dbdb      *mgo.Database
	dblogc    *mgo.Collection
)

func dbConnect() error {
	if dbsession != nil {
		return nil
	}

	var mgourl string
	if Configuration.MGOUsername != "" {
		mgourl = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s",
			Configuration.MGOUsername, Configuration.MGOPassword,
			Configuration.MGOHost, Configuration.MGOPort,
			Configuration.MGODBName)
	} else {
		mgourl = fmt.Sprintf("mongodb://%s:%s/%s",
			Configuration.MGOHost, Configuration.MGOPort,
			Configuration.MGODBName)
	}

	log.Debug("Connecting to database %s", mgourl)

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
