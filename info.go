package main

import (
	"encoding/json"
	"fmt"
	"github.com/RangelReale/appstatsd/infohttp"
	"github.com/gorilla/mux"
	"net/http"
)

func ServerInfo() {
	r := mux.NewRouter()

	r.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		session, err := DBConnectClone()
		if err != nil {
			handleError(fmt.Errorf("Error reading data: %s", err), w, r)
			return
		}
		defer session.Close()

		db := session.DB(Configuration.MGODBName)

		if err := infohttp.HandleLog(db, w, r); err != nil {
			handleError(err, w, r)
		}
	})

	r.HandleFunc("/stats/{process}", func(w http.ResponseWriter, r *http.Request) {
		session, err := DBConnectClone()
		if err != nil {
			handleError(fmt.Errorf("Error reading data: %s", err), w, r)
			return
		}
		defer session.Close()

		db := session.DB(Configuration.MGODBName)

		vars := mux.Vars(r)
		process := vars["process"]

		if err := infohttp.HandleStats(process, db, w, r); err != nil {
			handleError(err, w, r)
		}
	})
	r.NotFoundHandler = http.HandlerFunc(handleNotFound)

	http.Handle("/", r)

	http.ListenAndServe(fmt.Sprintf("%s:%d", Configuration.ListenHost, Configuration.InfoPort), nil)
}

func handleError(err error, w http.ResponseWriter, r *http.Request) {
	log.Error("Info error: %s", err)

	stenc, eerr := json.Marshal(infohttp.InfoResponse{ErrorCode: 1, ErrorMessage: err.Error()})
	if eerr != nil {
		w.Write([]byte(fmt.Sprintf("Error: %s", err)))
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.Write(stenc)
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	log.Error("URL not found: %s", r.URL)

	stenc, eerr := json.Marshal(infohttp.InfoResponse{ErrorCode: 404, ErrorMessage: "Not found"})
	if eerr != nil {
		w.Write([]byte("Error: Not found"))
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.Write(stenc)
}
