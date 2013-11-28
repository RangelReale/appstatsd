package main

import (
	"fmt"
	"github.com/RangelReale/epochdate"
	"strings"
	"time"
)

type SDayCollect struct {
	Data   string
	Result []map[string]interface{}
	Import map[string]string
}

func NewSDayCollect(data string) *SDayCollect {
	return &SDayCollect{
		Data:   data,
		Result: make([]map[string]interface{}, 0),
		Import: make(map[string]string),
	}
}

func (s *SDayCollect) AddImport(value string, name string) {
	s.Import[value] = name

	if strings.HasPrefix(value, "t_") {
		n := "tc_" + strings.TrimPrefix(value, "t_")
		s.Import[n] = n
	}
}

func (s *SDayCollect) setImport(dest map[string]interface{}, values map[string]interface{}) {
	for iiv, iin := range s.Import {
		if values != nil {
			if v, ok := values[iiv]; ok {
				dest[iin] = v
			} else {
				dest[iin] = 0
			}
		} else {
			dest[iin] = 0
		}
	}
}

func (s *SDayCollect) EmptyDay(date string) {
	cdt, _ := epochdate.Parse(epochdate.RFC3339, date)
	switch s.Data {
	case "hour":
		for di := 0; di < 24; di++ {
			if cdt.UTCTime(di, 0, 0, 0).After(time.Now().UTC()) {
				return
			}
			dy := make(map[string]interface{})
			dy["date"] = date
			dy["hour"] = di
			s.setImport(dy, nil)
			s.Result = append(s.Result, dy)
		}
	case "minute":
		for di := 0; di < 24; di++ {
			for mi := 0; mi < 4; mi++ {
				if cdt.UTCTime(di, mi*15, 0, 0).After(time.Now().UTC()) {
					return
				}
				dy := make(map[string]interface{})
				dy["date"] = date
				dy["hour"] = di
				dy["minute"] = mi * 15
				s.setImport(dy, nil)
				s.Result = append(s.Result, dy)
			}
		}
	default:
		dy := make(map[string]interface{})
		dy["date"] = date
		s.setImport(dy, nil)
		s.Result = append(s.Result, dy)
	}
}

func (s *SDayCollect) ValueDay(date string, value map[string]interface{}) {
	cdt, _ := epochdate.Parse(epochdate.RFC3339, date)

	switch s.Data {
	case "hour":
		hvalue := value["_hr"].(map[string]interface{})
		for di := 0; di < 24; di++ {
			if cdt.UTCTime(di, 0, 0, 0).After(time.Now().UTC()) {
				return
			}
			dy := make(map[string]interface{})
			dy["date"] = date
			dy["hour"] = di
			s.setImport(dy, nil)

			hf, ok := hvalue[fmt.Sprintf("h_%d", di)]
			if ok {
				s.setImport(dy, hf.(map[string]interface{}))
			}
			s.Result = append(s.Result, dy)
		}
	case "minute":
		hvalue := value["_hr"].(map[string]interface{})
		for di := 0; di < 24; di++ {
			var mvalue map[string]interface{}

			hf, ok := hvalue[fmt.Sprintf("h_%d", di)]
			if ok {
				mintf, mok := hf.(map[string]interface{})["mn"]
				if mok {
					mvalue = mintf.(map[string]interface{})
				}
			}
			for mi := 0; mi < 4; mi++ {
				if cdt.UTCTime(di, mi*15, 0, 0).After(time.Now().UTC()) {
					return
				}

				dy := make(map[string]interface{})
				dy["date"] = date
				dy["hour"] = di
				dy["minute"] = mi * 15
				s.setImport(dy, nil)

				if mvalue != nil {
					mf, mfok := mvalue[fmt.Sprintf("m_%d", mi*15)]
					if mfok {
						s.setImport(dy, mf.(map[string]interface{}))
					}
				}
				s.Result = append(s.Result, dy)
			}
		}
	default:
		dy := make(map[string]interface{})
		dy["date"] = date
		s.setImport(dy, value["_dy"].(map[string]interface{}))
		s.Result = append(s.Result, dy)
	}
}
