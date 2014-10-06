package info

import (
	"code.google.com/p/plotinum/plot"
	"fmt"
	"github.com/RangelReale/appstatsd/data"
	"github.com/RangelReale/epochdate"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"strings"
)

type StatsQuery struct {
	Process string
	Data    []string
	Period  string
	Filters map[string]string
	Groups  []string
	Amount  int
	App     string
}

type StatsQueryResult struct {
	StartDate epochdate.Date
	EndDate   epochdate.Date

	// my be *InfoResult or *InfoResultGroup
	Result interface{}
}

func QueryStats(db *mgo.Database, statsquery *StatsQuery) (*StatsQueryResult, error) {
	// check parameters
	if statsquery.Process == "" || statsquery.Data == nil || len(statsquery.Data) == 0 {
		return nil, fmt.Errorf("Required parameter not sent")
	}
	if statsquery.Groups == nil {
		statsquery.Groups = make([]string, 0)
	}

	// sanitize
	if !data.ValidateValueName(statsquery.Process) {
		return nil, fmt.Errorf("Invalid process name - name not validated: %s", statsquery.Process)
	}
	if statsquery.App != "" && !data.ValidateName(statsquery.App) {
		return nil, fmt.Errorf("Invalid app name - name not validated: %s", statsquery.App)
	}

	if statsquery.Amount < 1 {
		statsquery.Amount = 1
	}

	// find collection
	cname := fmt.Sprintf("stat_%s", statsquery.Process)
	if statsquery.App != "" {
		cname += "-app"
	}

	if !infoCollectionExists(db, cname) {
		return nil, fmt.Errorf("Process not found: %s", statsquery.Process)
	}

	c := db.C(cname)
	if statsquery.App != "" {
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
	curdate := epochdate.TodayUTC() - epochdate.Date(statsquery.Amount) + 1
	startdate := curdate

	// build mongodb filter
	filter := bson.M{"_dt": bson.M{"$gte": startdate.String()}}
	if statsquery.App != "" {
		filter["_app"] = statsquery.App
	}

	if statsquery.Filters != nil {
		for pn, pv := range statsquery.Filters {
			// sanitize
			if !data.ValidateValueName(pn) {
				return nil, fmt.Errorf("Invalid filter name - name not validated: %s", pn)
			}
			filter[pn] = pv
		}
	}

	querysort := []string{"_dt"}
	if len(statsquery.Groups) > 0 {
		for _, g := range statsquery.Groups {
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
		if len(statsquery.Groups) > 0 {
			for _, g := range statsquery.Groups {
				if gv, ok := fdata[g]; ok {
					curgroup = curgroup + "::" + fmt.Sprintf("%v", gv)
				} else {
					return nil, fmt.Errorf("No such field %s", g)
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
			curdate = epochdate.TodayUTC() - epochdate.Date(statsquery.Amount) + 1
			startdate = curdate

			lastgroup = curgroup
		}

		ginfo, sdok := groupcollect[curgroup]
		if !sdok {
			// stats collector, fills empty periods with 0
			scollect := NewSDayCollect(statsquery.Period)
			for _, ditem := range statsquery.Data {
				// add data - output name is equals data name
				scollect.AddImport(ditem, ditem)
			}
			ginfo = &InfoGroupInfo{GroupId: curgroup, Groups: make(map[string]interface{}), Collect: scollect}
			groupcollect[curgroup] = ginfo

			for _, g := range statsquery.Groups {
				if gv, ok := fdata[g]; ok {
					ginfo.Groups[g] = gv
				} else {
					return nil, fmt.Errorf("No such field %s", g)
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
		return nil, fmt.Errorf("Error reading data: %s", err)
	}

	if lastgroup != "" {
		// fill until today
		ginfo, _ := groupcollect[lastgroup]

		for curdate.Before(epochdate.TodayUTC()) {
			ginfo.Collect.EmptyDay(curdate.String())
			curdate += 1
		}
	}

	var res interface{}
	if len(statsquery.Groups) > 0 {
		resgroup := &InfoResultGroup{Group: make([]*InfoResultGroupItem, 0)}
		for _, gv := range groupcollect {
			resgroup.Group = append(resgroup.Group, &InfoResultGroupItem{GroupId: gv.GroupId, Groups: gv.Groups, InfoResult: &InfoResult{List: gv.Collect.Result}})
		}
		res = resgroup
	} else {
		resinfo := &InfoResult{List: nil}
		if ri, ok := groupcollect[""]; ok {
			resinfo.List = ri.Collect.Result
		} else {
			resinfo.List = nil
		}
		res = resinfo
	}

	return &StatsQueryResult{
		StartDate: startdate,
		EndDate:   curdate,
		Result:    res,
	}, nil
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

type InfoResultGroup struct {
	Group []*InfoResultGroupItem `json:"group"`
}

type InfoResultGroupItem struct {
	*InfoResult
	GroupId string                 `json:"-"`
	Groups  map[string]interface{} `json:"groups"`
	//Item   *InfoResult            `json:"item"`
}

type InfoGroupInfo struct {
	GroupId string
	Groups  map[string]interface{}
	Collect *SDayCollect
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

	// output average for timing values
	if strings.HasPrefix(s.plotItem, "t_") {
		sv := s.fieldValue(index, "tc_"+strings.TrimPrefix(s.plotItem, "t_"))
		if sv > 0 {
			v = v / sv
		} else {
			v = 0
		}
	}
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
