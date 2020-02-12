package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/mmirolim/gpp/macro"
)

func main() {
	fmt.Println("Coronavirus 2019 Time Series Data")
	var dates []time.Time
	var records []Record
	err := macro.Try_μ(func() error {
		resp, _ := http.Get(link)
		r := csv.NewReader(resp.Body)
		recs, _ := r.ReadAll()
		// get dates from header
		macro.NewSeq_μ(recs[0][4:]).Map(func(d string) time.Time {
			dateTime, _ := time.Parse(dateFormat, d)
			return dateTime
		}).Ret(&dates)
		recs = recs[1:]
		macro.NewSeq_μ(recs).Map(NewRecord).Ret(&records)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	totalByCountry := map[string]int{}
	totalCases := 0
	macro.NewSeq_μ(records).
		Reduce(&totalByCountry, func(acc mapStrInt, r Record) mapStrInt {
			acc[r.Country] += int(r.Dates[len(r.Dates)-1].Number)
			return acc
		}).
		Reduce(&totalCases, func(acc int, r Record) int {
			return acc + int(r.Dates[len(r.Dates)-1].Number)
		})

	macro.Log_μ(">> Total Number of Cases", totalCases)
	type casesByDate struct {
		date  time.Time
		cases int
	}
	casesByDates := make([]casesByDate, len(dates))
	macro.NewSeq_μ(casesByDates).Map(func(v casesByDate, i int) casesByDate {
		macro.NewSeq_μ(records).
			Reduce(&casesByDates[i].cases, func(acc int, r Record) int {
				return acc + int(r.Dates[i].Number)
			})
		casesByDates[i].date = dates[i]
		return casesByDates[i]
	})
	var cbd []string
	macro.NewSeq_μ(casesByDates).Map(func(v casesByDate, i int) string {
		bar := make([]byte, int(math.Log2(float64(casesByDates[i].cases))))
		macro.NewSeq_μ(bar).Map(func(ch byte, i int) byte {
			bar[i] = '*'
			return bar[i]
		})
		return fmt.Sprintf("%s %s %d",
			casesByDates[i].date.Format("01/02/06"),
			string(bar), casesByDates[i].cases)
	}).Ret(&cbd)
	macro.Log_μ(">> Log Scale")
	macro.PrintSlice_μ(cbd)

	var countries []string
	macro.MapKeys_μ(&countries, totalByCountry)
	sort.Strings(countries)
	macro.Log_μ(">> Sorted by country")
	macro.PrintMapKeys_μ(countries, totalByCountry)

	var css []countryCases
	macro.MapToSlice_μ(&css, totalByCountry,
		func(k string, num int) countryCases { return countryCases{k, num} })

	sort.Slice(css, func(i, j int) bool {
		return css[i].Cases > css[j].Cases
	})
	macro.Log_μ(">> Sorted by number of cases")
	macro.PrintSlice_μ(css)
}

const (
	link       = "https://raw.githubusercontent.com/CSSEGISandData/2019-nCoV/master/time_series/time_series_2019-ncov-Confirmed.csv"
	dateFormat = "1/2/06 15:04"
)

type mapStrInt map[string]int
type countryCases struct {
	Country string
	Cases   int
}

type Record struct {
	Country   string
	Province  string
	Lat, Long float64
	Dates     []Date
}

type Date struct {
	Date   time.Time
	Number float64
}

func NewRecord(rec []string) Record {
	record := Record{
		Province: rec[0], Country: rec[1],
	}
	err := macro.Try_μ(func() error {
		record.Lat, _ = strconv.ParseFloat(rec[2], 64)
		record.Long, _ = strconv.ParseFloat(rec[3], 64)
		for i := 4; i < len(rec); i++ {
			date := Date{}
			if rec[i] != "" {
				date.Number, _ = strconv.ParseFloat(rec[i], 64)
			}
			record.Dates = append(record.Dates, date)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return record
}
