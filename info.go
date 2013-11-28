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
	"github.com/RangelReale/epochdate"
	"github.com/gorilla/mux"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"net/http"
	"strconv"
	"strings"
)

type InfoResponse struct {
	ErrorCode    int32       `json:"error_code"`
	ErrorMessage string      `json:"error_message,omitempty"`
	Data         interface{} `json:"data"`
}

type InfoResult struct {
	List     []map[string]interface{} `json:"list"`
	plotItem string                   `json:"-"`
}

func ServerInfo() {
	r := mux.NewRouter()
	r.HandleFunc("/stats/{process}", func(w http.ResponseWriter, r *http.Request) {
		if err := handleStats(w, r); err != nil {
			log.Error("Error: %s", err)

			stenc, eerr := json.Marshal(InfoResponse{ErrorCode: 1, ErrorMessage: err.Error()})
			if eerr != nil {
				w.Write([]byte(fmt.Sprintf("Error: %s", err)))
				return
			}

			w.Header().Add("Content-Type", "application/json; charset=utf-8")
			w.Write(stenc)

		}
	})
	r.NotFoundHandler = http.HandlerFunc(handleNotFound)

	http.Handle("/", r)

	http.ListenAndServe(fmt.Sprintf(":%d", Configuration.InfoPort), nil)
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	log.Error("URL not found: %s", r.URL)

	w.WriteHeader(404)
	w.Write([]byte("Not found"))
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

	// parameters - default is first from comment
	period := r.Form.Get("period") // day, hour, minute
	output := r.Form.Get("output") // json, chart
	//fillempty := r.Form.Get("fillempty") // 1, 0

	//app := r.Form.Get("app")
	pdata := r.Form.Get("data")
	if pdata == "" {
		return errors.New("Data parameter is required")
	}

	data := strings.Split(pdata, ",")

	amount := 2
	if r.Form.Get("amount") != "" {
		pamount, err := strconv.ParseInt(r.Form.Get("amount"), 10, 16)
		if err == nil {
			amount = int(pamount)
		}
	}

	cname := fmt.Sprintf("_a_%s", process)

	if !infoCollectionExists(db, cname) {
		return errors.New("Process not found")
	}

	c := db.C(cname)
	c.EnsureIndex(mgo.Index{
		Key:        []string{"_dt"},
		Background: true,
		Sparse:     true,
	})

	// start time
	curdate := epochdate.TodayUTC() - epochdate.Date(amount) + 1
	startdate := curdate

	filter := bson.M{"_dt": bson.M{"$gte": startdate.String()}}
	for pn, _ := range r.Form {
		if strings.HasPrefix(pn, "f_") {
			pname := strings.TrimPrefix(pn, "f_")
			if strings.HasPrefix(pname, "_") {
				return errors.New(fmt.Sprintf("Error filter param: %s", pname))
			}

			filter[pname] = r.Form.Get(pn)
		}
	}

	query := c.Find(filter).Sort("_dt").Limit(amount).Iter()

	scollect := NewSDayCollect(period)
	for _, ditem := range data {
		scollect.AddImport(ditem, ditem)
	}

	fdata := make(map[string]interface{})
	for query.Next(&fdata) {
		//log.Debug("%+v", fdata)

		datadate, _ := epochdate.Parse(epochdate.RFC3339, fdata["_dt"].(string))
		for curdate.Before(datadate) {
			scollect.EmptyDay(curdate.String())
			curdate += 1
		}
		scollect.ValueDay(curdate.String(), fdata)

		curdate += 1
	}

	if err := query.Close(); err != nil {
		return errors.New(fmt.Sprintf("Error reading data: %s", err))
	}

	for curdate.Before(epochdate.TodayUTC()) {
		scollect.EmptyDay(curdate.String())
		curdate += 1
	}

	res := InfoResult{List: scollect.Result}

	if output != "chart" {
		stenc, err := json.Marshal(InfoResponse{ErrorCode: 0, Data: res})
		if err != nil {
			return errors.New(fmt.Sprintf("Error encoding json data: %s", err))
		}

		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.Write(stenc)
	} else {

		p, err := plot.New()
		if err != nil {
			return err
		}

		p.Title.Text = fmt.Sprintf("Chart %s - %s", pdata, period)
		p.X.Label.Text = fmt.Sprintf("Day (%s to %s)", startdate.String(), curdate.String())
		//p.X.Tick.Marker = plot.ConstantTicks(res.Ticks())

		for didx, ditem := range data {
			res.SetPlotItem(ditem)
			err = InfoAddLinePoints(p, didx, res)
			if err != nil {
				return err
			}
		}

		w.Header().Add("Content-Type", "image/png")

		width := vg.Length(640)
		height := vg.Length(480)
		c := vgimg.PngCanvas{vgimg.New(width, height)}
		p.Draw(plot.MakeDrawArea(c))
		c.WriteTo(w)
	}

	return nil
}

func infoCollectionExists(db *mgo.Database, name string) bool {
	list, err := db.CollectionNames()
	if err != nil {
		log.Error("Error listing info collections: %s", err)
		return false
	}

	for _, l := range list {
		//if strings.HasPrefix(l, fmt.Sprintf("_a_%s", name)) {
		if l == name {
			return true
		}
	}
	return false
}

func (s *InfoResult) SetPlotItem(item string) {
	s.plotItem = item
}

// implement plotter.XYer
func (s InfoResult) Len() int {
	return len(s.List)
}

func (s InfoResult) fieldValue(index int, fieldname string) float64 {
	r := s.List[index][fieldname]

	switch v := r.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	}
	return -1
}

// XY returns an x, y pair.
func (s InfoResult) XY(index int) (x, y float64) {
	v := s.fieldValue(index, s.plotItem)
	/*
		if s.plotItem == "duration" {
			sv := s.fieldValue(index, "count")
			if sv > 0 {
				v = v / sv
			} else {
				v = 0
			}
		}
	*/
	return float64(index), v
}

func (s InfoResult) Ticks() []plot.Tick {
	ret := make([]plot.Tick, len(s.List))
	for x, i := range s.List {
		ret = append(ret, plot.Tick{Value: float64(x), Label: i["date"].(string)})
	}
	return ret
}

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
