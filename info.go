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

// Response header
type InfoResponse struct {
	ErrorCode    int32  `json:"error_code"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Must always be a struct or map, CANNOT be array
	Data interface{} `json:"data"`
}

type InfoResultGroup struct {
	Group []*InfoResultGroupItem `json:"group"`
}

type InfoResultGroupItem struct {
	*InfoResult
	GroupId string                 `json:"-"`
	Groups  map[string]interface{} `json:"groups"`
	//Item   *InfoResult            `json:"item"`
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

type InfoGroupInfo struct {
	GroupId string
	Groups  map[string]interface{}
	Collect *SDayCollect
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

	fdata := make([]*LogData, 0, 100)

	query := c_log.Find(nil).Sort("-dt").Limit(amount).Iter()
	flog := &LogData{}

	for query.Next(&flog) {
		fdata = append(fdata, flog)
	}

	if err := query.Close(); err != nil {
		return errors.New(fmt.Sprintf("Error reading data: %s", err))
	}

	res := InfoResultRaw{List: fdata}

	// output json data
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

	// parameters - default is first from comment
	period := r.Form.Get("period") // day, hour, minute
	output := r.Form.Get("output") // json, chart
	//fillempty := r.Form.Get("fillempty") // 1, 0

	app := r.Form.Get("app")
	pgroup := r.Form.Get("group")
	group := []string{}
	if pgroup != "" {
		group = strings.Split(pgroup, ",")
	}
	pdata := r.Form.Get("data")
	if pdata == "" {
		return errors.New("Data parameter is required")
	}
	data := strings.Split(pdata, ",")

	// amount of days
	amount := 2
	if r.Form.Get("amount") != "" {
		pamount, err := strconv.ParseInt(r.Form.Get("amount"), 10, 16)
		if err == nil {
			amount = int(pamount)
		}
	}

	cname := fmt.Sprintf("_a_%s", process)
	if app != "" {
		cname += "_app"
	}

	if !infoCollectionExists(db, cname) {
		return errors.New("Process not found")
	}

	c := db.C(cname)
	if app != "" {
		c.EnsureIndex(mgo.Index{
			Key:        []string{"_dt", "_app"},
			Background: true,
			Sparse:     true,
		})
	} else {
		c.EnsureIndex(mgo.Index{
			Key:        []string{"_dt"},
			Background: true,
			Sparse:     true,
		})
	}

	// start time
	curdate := epochdate.TodayUTC() - epochdate.Date(amount) + 1
	startdate := curdate

	// build mongodb filter
	filter := bson.M{"_dt": bson.M{"$gte": startdate.String()}}
	if app != "" {
		filter["_app"] = app
	}

	for pn, _ := range r.Form {
		// each parameter prefixed with f_ becomes a query parameter
		if strings.HasPrefix(pn, "f_") {
			pname := strings.TrimPrefix(pn, "f_")
			if strings.HasPrefix(pname, "_") {
				return errors.New(fmt.Sprintf("Invalid filter param - cannot start with underline: %s", pname))
			}

			filter[pname] = r.Form.Get(pn)
		}
	}

	querysort := []string{"_dt"}
	if len(group) > 0 {
		for _, g := range group {
			querysort = append(querysort, g)
		}
	}

	query := c.Find(filter).Sort(querysort...).Iter()

	groupcollect := make(map[string]*InfoGroupInfo, 0)
	lastgroup := ""

	fdata := make(map[string]interface{})
	for query.Next(&fdata) {
		// build group string
		curgroup := ""
		if len(group) > 0 {
			for _, g := range group {
				if gv, ok := fdata[g]; ok {
					curgroup = curgroup + "::" + fmt.Sprintf("%v", gv)
				} else {
					return errors.New(fmt.Sprintf("No such field %s", g))
				}
			}
		}

		// reset dates if changed group
		if curgroup != lastgroup {
			if lastgroup != "" {
				ginfo, _ := groupcollect[lastgroup]

				// fill until today
				for curdate.Before(epochdate.TodayUTC()) {
					ginfo.Collect.EmptyDay(curdate.String())
					curdate += 1
				}
			}

			// start time
			curdate = epochdate.TodayUTC() - epochdate.Date(amount) + 1
			startdate = curdate

			lastgroup = curgroup
		}

		ginfo, sdok := groupcollect[curgroup]
		if !sdok {
			// stats collector, fills empty periods with 0
			scollect := NewSDayCollect(period)
			for _, ditem := range data {
				// add data - output name is equals data name
				scollect.AddImport(ditem, ditem)
			}
			ginfo = &InfoGroupInfo{GroupId: curgroup, Groups: make(map[string]interface{}), Collect: scollect}
			groupcollect[curgroup] = ginfo

			for _, g := range group {
				if gv, ok := fdata[g]; ok {
					ginfo.Groups[g] = gv
				} else {
					return errors.New(fmt.Sprintf("No such field %s", g))
				}
			}
		}

		datadate, _ := epochdate.Parse(epochdate.RFC3339, fdata["_dt"].(string))
		for curdate.Before(datadate) {
			// fill empty days
			ginfo.Collect.EmptyDay(curdate.String())
			curdate += 1
		}

		// fill day from data
		ginfo.Collect.ValueDay(curdate.String(), fdata)

		curdate += 1
	}

	if err := query.Close(); err != nil {
		return errors.New(fmt.Sprintf("Error reading data: %s", err))
	}

	if lastgroup != "" {
		// fill until today
		ginfo, _ := groupcollect[lastgroup]

		for curdate.Before(epochdate.TodayUTC()) {
			ginfo.Collect.EmptyDay(curdate.String())
			curdate += 1
		}
	}

	var resinfo *InfoResult
	var resgroup *InfoResultGroup
	var res interface{}
	if len(group) > 0 {
		resgroup = &InfoResultGroup{Group: make([]*InfoResultGroupItem, 0)}
		for _, gv := range groupcollect {
			resgroup.Group = append(resgroup.Group, &InfoResultGroupItem{GroupId: gv.GroupId, Groups: gv.Groups, InfoResult: &InfoResult{List: gv.Collect.Result}})
		}
		res = resgroup
	} else {
		resinfo = &InfoResult{List: nil}
		if ri, ok := groupcollect[""]; ok {
			resinfo.List = ri.Collect.Result
		} else {
			resinfo.List = nil
		}
		res = resinfo
	}

	if output != "chart" {
		// output json data
		stenc, err := json.Marshal(InfoResponse{ErrorCode: 0, Data: res})
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

		p.Title.Text = fmt.Sprintf("Chart %s - %s", pdata, period)
		p.X.Label.Text = fmt.Sprintf("Day (%s to %s)", startdate.String(), curdate.String())
		//p.X.Tick.Marker = plot.ConstantTicks(res.Ticks())

		for didx, ditem := range data {
			if resinfo != nil {
				resinfo.SetPlotItem(ditem)
				err = InfoAddLinePoints(p, didx, ditem, resinfo)
				if err != nil {
					return err
				}
			} else if resgroup != nil {
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

// Checks if a collection exists on the MongoDB database
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

// Sets item to be used on plotter.XYer interface
func (s *InfoResult) SetPlotItem(item string) {
	s.plotItem = item
}

// implement plotter.XYer
func (s InfoResult) Len() int {
	return len(s.List)
}

// must convert values to float64
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

// Return X axis tick marks
func (s InfoResult) Ticks() []plot.Tick {
	ret := make([]plot.Tick, len(s.List))
	for x, i := range s.List {
		ret = append(ret, plot.Tick{Value: float64(x), Label: i["date"].(string)})
	}
	return ret
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
