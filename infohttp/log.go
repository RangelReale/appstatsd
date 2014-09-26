package infohttp

import (
	"encoding/json"
	"fmt"
	"github.com/RangelReale/appstatsd/info"
	"gopkg.in/mgo.v2"
	"net/http"
	"strconv"
)

func HandleLog(db *mgo.Database, w http.ResponseWriter, r *http.Request) error {
	// amount of records
	amount := 100
	if r.Form.Get("amount") != "" {
		pamount, err := strconv.ParseInt(r.Form.Get("amount"), 10, 16)
		if err == nil {
			amount = int(pamount)
		}
	}

	// do query
	fdata, err := info.QueryLog(db, &info.LogQuery{Amount: amount, App: ""})
	if err != nil {
		return fmt.Errorf("Error reading data: %s", err)
	}

	// output json data
	stenc, err := json.Marshal(
		InfoResponse{
			ErrorCode: 0,
			Data: InfoResultRaw{
				List: fdata,
			},
		})
	if err != nil {
		return fmt.Errorf("Error encoding json data: %s", err)
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.Write(stenc)

	return nil
}
