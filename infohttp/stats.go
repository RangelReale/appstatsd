package infohttp

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"

	"code.google.com/p/plotinum/plot"
	"code.google.com/p/plotinum/plotter"
	"code.google.com/p/plotinum/plotutil"
	"code.google.com/p/plotinum/vg"
	"code.google.com/p/plotinum/vg/vgimg"
	"github.com/RangelReale/appstatsd/info"
	"gopkg.in/mgo.v2"
)

func HandleStats(process string, db *mgo.Database, w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

	amount := 2
	chartwidth := 800
	chartheight := 600

	if r.Form.Get("amount") != "" {
		pamount, err := strconv.ParseInt(r.Form.Get("amount"), 10, 16)
		if err == nil {
			amount = int(pamount)
		}
	}
	if r.Form.Get("chartwidth") != "" {
		pchartwidth, err := strconv.ParseInt(r.Form.Get("chartwidth"), 10, 16)
		if err == nil {
			chartwidth = int(pchartwidth)
		}
	}
	if r.Form.Get("chartheight") != "" {
		pchartheight, err := strconv.ParseInt(r.Form.Get("chartheight"), 10, 16)
		if err == nil {
			chartheight = int(pchartheight)
		}
	}
	if chartwidth < 40 {
		chartwidth = 40
	}
	if chartheight < 40 {
		chartheight = 40
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

	// filters
	for fname, _ := range r.Form {
		if strings.HasPrefix(fname, "f_") {
			q.Filters[strings.TrimPrefix(fname, "f_")] = r.Form.Get(fname)
		}
	}

	res, err := info.QueryStats(db, q)
	if err != nil {
		return err
	}

	if output == "chart" {
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

		width := vg.Length(chartwidth)
		height := vg.Length(chartheight)
		c := vgimg.PngCanvas{vgimg.New(width, height)}
		p.Draw(plot.MakeDrawArea(c))
		c.WriteTo(w)
	} else if output == "table" || output == "atable" {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")

		w.Write([]byte("<table border=\"1\">"))

		for _, ditem := range q.Data {

			w.Write([]byte("<tr>"))
			w.Write([]byte(fmt.Sprintf("<td><b>%s</b></td>", html.EscapeString(ditem))))

			if resinfo, ok := res.Result.(*info.InfoResult); ok {
				resinfo.SetPlotItem(ditem)
				for ridx, rdata := range resinfo.List {
					var dvalue string
					if output == "atable" {
						_, dd := resinfo.XY(ridx)
						if dd != 0 {
							dvalue = fmt.Sprintf("%.1f", dd)
						} else {
							dvalue = "0"
						}
					} else {
						dvalue = fmt.Sprintf("%v", rdata[ditem])
					}
					if dvalue == "0" {
						dvalue = "-"
					}

					w.Write([]byte(fmt.Sprintf("<td align=\"center\">%s</td>", html.EscapeString(dvalue))))
				}

				w.Write([]byte("</tr>"))
			} else if resgroup, ok := res.Result.(*info.InfoResultGroup); ok {
				w.Write([]byte("</tr>"))

				for _, rg := range resgroup.Group {
					w.Write([]byte(fmt.Sprintf("<tr><td>%s</td>", rg.GroupId)))

					rg.SetPlotItem(ditem)
					for ridx, rdata := range rg.List {
						var dvalue string
						if output == "atable" {
							_, dd := rg.XY(ridx)
							if dd != 0 {
								dvalue = fmt.Sprintf("%.1f", dd)
							} else {
								dvalue = "0"
							}
						} else {
							dvalue = fmt.Sprintf("%v", rdata[ditem])
						}
						if dvalue == "0" {
							dvalue = "-"
						}

						w.Write([]byte(fmt.Sprintf("<td align=\"center\">%s</td>", html.EscapeString(dvalue))))
					}

					w.Write([]byte("</tr>"))
				}
			}
		}

		w.Write([]byte("</table>"))
	} else {
		// output json data
		stenc, err := json.Marshal(InfoResponse{ErrorCode: 0, Data: res.Result})
		if err != nil {
			return fmt.Errorf("Error encoding json data: %s", err)
		}

		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.Write(stenc)

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
