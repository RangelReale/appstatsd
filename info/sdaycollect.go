package info

import (
	"fmt"
	"github.com/RangelReale/epochdate"
	"strings"
	"time"
)

// Fills empty periods with zeroes
type SDayCollect struct {
	Data string // day, hour, minute
	//Result []map[string]interface{} // Collected result
	Import map[string]string

	resultOrder []string
	resultData  map[string]map[string]interface{} // Collected result

	isinit bool
}

func NewSDayCollect(data string) *SDayCollect {
	s := &SDayCollect{
		Data: data,
		//Result: make([]map[string]interface{}, 0),
		Import: make(map[string]string),

		resultOrder: make([]string, 0),
		resultData:  make(map[string]map[string]interface{}),

		isinit: true,
	}
	return s
}

// Add a field to import, with possibly alternate output name
func (s *SDayCollect) AddImport(value string, name string) {
	if !s.isinit {
		panic("Cannot AddImport after init")
	}

	s.Import[value] = name

	if strings.HasPrefix(value, "t_") {
		n := "tc_" + strings.TrimPrefix(value, "t_")
		s.Import[n] = n
	} else if strings.HasPrefix(value, "g_") {
		n := "gc_" + strings.TrimPrefix(value, "g_")
		s.Import[n] = n
	}
}

func (s *SDayCollect) Init(startdate epochdate.Date, enddate epochdate.Date) {
	if !s.isinit {
		panic("Not in init mode")
	}
	if enddate.Before(startdate) {
		panic("Startdate must be before enddate")
	}

	filldate := startdate
	for filldate.Before(enddate + 1) {
		s.EmptyDay(filldate)
		filldate += 1
	}

	s.isinit = false
}

// Write imported data, using zeroes if not found
func (s *SDayCollect) addImportData(dest map[string]interface{}, values map[string]interface{}) {
	for iiv, iin := range s.Import {
		destv := float64(0)
		if values != nil {
			if v, ok := values[iiv]; ok {
				destv = v.(float64)
			}
		}
		if dval, dok := dest[iin]; dok {
			dnval := dval.(float64) + destv
			dest[iin] = dnval
		} else {
			dest[iin] = destv
		}
	}
}

// Fills data with zeroes for empty day
func (s *SDayCollect) EmptyDay(date epochdate.Date) {
	s.ValueDay(date, nil)
}

/*
func (s *SDayCollect) EmptyDay(date epochdate.Date) {
	//cdt, _ := epochdate.Parse(epochdate.RFC3339, date)
	switch s.Data {
	case "hour":
		for di := 0; di < 24; di++ {
			if date.UTCTime(di, 0, 0, 0).After(time.Now().UTC()) {
				return
			}
			dy := make(map[string]interface{})
			dy["date"] = date.String()
			dy["hour"] = di
			//s.addImportData(dy, nil)
			//s.Result = append(s.Result, dy)
			s.setData(fmt.Sprintf("%s-%d", date.String(), di), nil)
		}
	case "minute":
		for di := 0; di < 24; di++ {
			// minutes in 15 minutes increments
			for mi := 0; mi < 4; mi++ {
				if date.UTCTime(di, mi*15, 0, 0).After(time.Now().UTC()) {
					return
				}
				dy := make(map[string]interface{})
				dy["date"] = date.String()
				dy["hour"] = di
				dy["minute"] = mi * 15
				s.addImportData(dy, nil)
				s.Result = append(s.Result, dy)
			}
		}
	default:
		dy := make(map[string]interface{})
		dy["date"] = date.String()
		s.addImportData(dy, nil)
		s.Result = append(s.Result, dy)
	}
}
*/

// Sets values for a day. Empty periods are zeroed
func (s *SDayCollect) ValueDay(date epochdate.Date, value map[string]interface{}) {
	switch s.Data {
	case "hour":
		for di := 0; di < 24; di++ {
			// don't add if in past
			if date.UTCTime(di, 0, 0, 0).After(time.Now().UTC()) {
				return
			}
			dy := make(map[string]interface{})
			dy["date"] = date.String()
			dy["hour"] = di

			var fd map[string]interface{}
			if value != nil {
				hvalue := value["_hr"].(map[string]interface{})
				hf, ok := hvalue[fmt.Sprintf("h_%d", di)]
				if ok {
					fd = hf.(map[string]interface{})
				}
			}
			s.addImportData(dy, fd)

			s.setData(fmt.Sprintf("%s@%d", date.String(), di), dy)
		}
	case "minute":
		for di := 0; di < 24; di++ {
			var mvalue map[string]interface{}

			if value != nil {
				hvalue := value["_hr"].(map[string]interface{})
				hf, ok := hvalue[fmt.Sprintf("h_%d", di)]
				if ok {
					mintf, mok := hf.(map[string]interface{})["mn"]
					if mok {
						mvalue = mintf.(map[string]interface{})
					}
				}
			}
			// minutes in 15 minutes increments
			for mi := 0; mi < 4; mi++ {
				if date.UTCTime(di, mi*15, 0, 0).After(time.Now().UTC()) {
					return
				}

				dy := make(map[string]interface{})
				dy["date"] = date.String()
				dy["hour"] = di
				dy["minute"] = mi * 15

				var fd map[string]interface{}
				if mvalue != nil {
					mf, mfok := mvalue[fmt.Sprintf("m_%d", mi*15)]
					if mfok {
						fd = mf.(map[string]interface{})
					}
				}
				s.addImportData(dy, fd)
				s.setData(fmt.Sprintf("%s@%d@%d", date.String(), di, mi*15), dy)
			}
		}
	default:
		dy := make(map[string]interface{})
		dy["date"] = date.String()

		var fd map[string]interface{}
		if value != nil {
			fd = value["_dy"].(map[string]interface{})
		}
		s.addImportData(dy, fd)
		s.setData(fmt.Sprintf("%s", date.String()), dy)
	}
}

func (s *SDayCollect) setData(datestr string, value map[string]interface{}) {
	//log.Printf("setData: %s", datestr)

	sd, sdok := s.resultData[datestr]
	if !sdok {
		if !s.isinit {
			panic("DateStr not valid " + datestr)
		}
		s.resultData[datestr] = value
		s.resultOrder = append(s.resultOrder, datestr)
	} else {
		s.addImportData(sd, value)
	}
}

func (s *SDayCollect) BuildResult() []map[string]interface{} {
	ret := make([]map[string]interface{}, 0)
	for _, rname := range s.resultOrder {
		ret = append(ret, s.resultData[rname])
	}
	return ret
}
