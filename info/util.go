package info

import (
	"gopkg.in/mgo.v2"
	"strings"
)

// Checks if a collection exists on the MongoDB database
func infoCollectionExists(db *mgo.Database, name string) bool {
	list, err := db.CollectionNames()
	if err != nil {
		return false
	}

	for _, l := range list {
		if l == name {
			return true
		}
	}
	return false
}

func SplitParams(params string) []string {
	if params == "" {
		return []string{}
	}

	return strings.Split(params, ",")
}
