package infohttp

import (
	"code.google.com/p/plotinum/plot"
	"code.google.com/p/plotinum/plotter"
	"code.google.com/p/plotinum/plotutil"
	"code.google.com/p/plotinum/vg"
	"code.google.com/p/plotinum/vg/vgimg"
	"encoding/json"
	"fmt"
	"github.com/RangelReale/appstatsd/info"
	"gopkg.in/mgo.v2"
	"net/http"
	"strconv"
)

func HandleStats(process string, db *mgo.Database, w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

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
			return fmt.Errorf("Error encoding json data: %s", err)
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
