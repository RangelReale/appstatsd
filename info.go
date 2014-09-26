package main

import (
	"code.google.com/p/plotinum/plot"
	"code.google.com/p/plotinum/plotter"
	"code.google.com/p/plotinum/plotutil"
	"code.google.com/p/plotinum/vg"
	"code.google.com/p/plotinum/vg/vgimg"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RangelReale/appstatsd/info"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

// Response header
type InfoResponse struct {
	ErrorCode    int32  `json:"error_code"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Must always be a struct or map, CANNOT be array
	Data interface{} `json:"data"`
}

type InfoResult struct {
	List []map[string]interface{} `json:"list"`

	// internal variable for chart generation
	plotItem string `json:"-"`
}

type InfoResultRaw struct {
	List interface{} `json:"list"`

	// internal variable for chart generation
	plotItem string `json:"-"`
}

func ServerInfo() {
	r := mux.NewRouter()

	r.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		if err := handleLog(w, r); err != nil {
			handleError(err, w, r)
		}
	})

	r.HandleFunc("/stats/{process}", func(w http.ResponseWriter, r *http.Request) {
		if err := handleStats(w, r); err != nil {
			handleError(err, w, r)
		}
	})
	r.NotFoundHandler = http.HandlerFunc(handleNotFound)

	http.Handle("/", r)

	http.ListenAndServe(fmt.Sprintf("%s:%d", Configuration.ListenHost, Configuration.InfoPort), nil)
}

func handleError(err error, w http.ResponseWriter, r *http.Request) {
	log.Error("Info error: %s", err)

	stenc, eerr := json.Marshal(InfoResponse{ErrorCode: 1, ErrorMessage: err.Error()})
	if eerr != nil {
		w.Write([]byte(fmt.Sprintf("Error: %s", err)))
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.Write(stenc)
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	log.Error("URL not found: %s", r.URL)

	w.WriteHeader(404)
	w.Write([]byte("Not found"))
}

func handleLog(w http.ResponseWriter, r *http.Request) error {
	session, err := DBConnectClone()
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading data: %s", err))
	}
	defer session.Close()

	db := session.DB(Configuration.MGODBName)

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
		return errors.New(fmt.Sprintf("Error reading data: %s", err))
	}

	// output json data
	res := InfoResultRaw{List: fdata}
	stenc, err := json.Marshal(InfoResponse{ErrorCode: 0, Data: res})
	if err != nil {
		return errors.New(fmt.Sprintf("Error encoding json data: %s", err))
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.Write(stenc)

	return nil
}

func handleStats(w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

	session, err := DBConnectClone()
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading data: %s", err))
	}
	defer session.Close()

	db := session.DB(Configuration.MGODBName)

	vars := mux.Vars(r)
	process := vars["process"]

	amount := 2
	if r.Form.Get("amount") != "" {
		pamount, err := strconv.ParseInt(r.Form.Get("amount"), 10, 16)
		if err == nil {
			amount = int(pamount)
		}
	}

	q := &info.StatsQuery{
		Process: process,
		Data:    info.SplitParams(r.Form.Get("data")),
		Period:  r.Form.Get("period"),
		Filters: make(map[string]string),
		Groups:  info.SplitParams(r.Form.Get("group")),
		Amount:  amount,
		App:     r.Form.Get("app"),
	}

	output := r.Form.Get("output") // json, chart

	res, err := info.QueryStats(db, q)
	if err != nil {
		return err
	}

	if output != "chart" {
		// output json data
		stenc, err := json.Marshal(InfoResponse{ErrorCode: 0, Data: res.Result})
		if err != nil {
			return errors.New(fmt.Sprintf("Error encoding json data: %s", err))
		}

		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.Write(stenc)
	} else {
		// output chart
		p, err := plot.New()
		if err != nil {
			return err
		}

		p.Title.Text = fmt.Sprintf("Chart %s - %s", r.Form.Get("data"), q.Period)
		p.X.Label.Text = fmt.Sprintf("Day (%s to %s)", res.StartDate.String(), res.EndDate.String())
		//p.X.Tick.Marker = plot.ConstantTicks(res.Ticks())

		for didx, ditem := range q.Data {
			if resinfo, ok := res.Result.(*info.InfoResult); ok {
				resinfo.SetPlotItem(ditem)
				err = InfoAddLinePoints(p, didx, ditem, resinfo)
				if err != nil {
					return err
				}
			} else if resgroup, ok := res.Result.(*info.InfoResultGroup); ok {
				for rgidx, rg := range resgroup.Group {
					rg.SetPlotItem(ditem)
					err = InfoAddLinePoints(p, didx*len(resgroup.Group)+rgidx, ditem+" - "+rg.GroupId, rg)
					if err != nil {
						return err
					}
				}
			}
		}

		w.Header().Add("Content-Type", "image/png")

		width := vg.Length(800)
		height := vg.Length(600)
		c := vgimg.PngCanvas{vgimg.New(width, height)}
		p.Draw(plot.MakeDrawArea(c))
		c.WriteTo(w)
	}

	return nil
}

// Add colored line to chart
func InfoAddLinePoints(plt *plot.Plot, color int, vs ...interface{}) error {
	var ps []plot.Plotter
	names := make(map[[2]plot.Thumbnailer]string)
	name := ""
	var i int = color
	for _, v := range vs {
		switch t := v.(type) {
		case string:
			name = t

		case plotter.XYer:
			l, s, err := plotter.NewLinePoints(t)
			if err != nil {
				return err
			}
			l.Color = plotutil.Color(i)
			l.Dashes = plotutil.Dashes(i)
			s.Color = plotutil.Color(i)
			s.Shape = plotutil.Shape(i)
			i++
			ps = append(ps, l, s)
			if name != "" {
				names[[2]plot.Thumbnailer{l, s}] = name
				name = ""
			}

		default:
			panic(fmt.Sprintf("AddLinePoints handles strings and plotter.XYers, got %T", t))
		}
	}
	plt.Add(ps...)
	for ps, n := range names {
		plt.Legend.Add(n, ps[0], ps[1])
	}
	return nil
}
