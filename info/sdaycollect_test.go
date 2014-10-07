package info

import (
	"github.com/RangelReale/epochdate"
	"log"
	"testing"
)

func TestCollect(t *testing.T) {
	startdate, _ := epochdate.NewFromDate(2001, 2, 10)
	enddate, _ := epochdate.NewFromDate(2001, 2, 11)

	c := NewSDayCollect("minute")
	c.AddImport("c_count", "c_count")
	c.Init(startdate, enddate)

	d1value := map[string]interface{}{
		"_hr": map[string]interface{}{
			"h_18": map[string]interface{}{
				"mn": map[string]interface{}{
					"m_15": map[string]interface{}{
						"c_count": float64(10),
					},
				},
			},
		},
	}

	c.ValueDay(startdate, d1value)

	for _, v := range c.BuildResult() {
		if v["c_count"].(float64) > 0 {
			log.Printf("%+v", v)
		}
	}
}
