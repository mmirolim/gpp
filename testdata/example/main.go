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
	fmt.Println("Coronavirus 2020 Time Series Data")

	var recordLines [][]string
	err := macro.Try_μ(func() error {
		resp, _ := http.Get(link)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get failed status %d", resp.StatusCode)
		}
		r := csv.NewReader(resp.Body)
		recordLines, _ = r.ReadAll()
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// get dates from header
	var dates []time.Time
	macro.NewSeq_μ(recordLines[0][4:]).Map(func(d string) time.Time {
		dateTime, _ := time.Parse(dateFormat, d)
		return dateTime
	}).Ret(&dates)

	// convert lines to records
	var records []Record
	recordLines = recordLines[1:]
	macro.NewSeq_μ(recordLines).Map(NewRecord).Ret(&records)

	totalByCountry := map[string]int{}
	totalCases := 0
	longestName := ""
	macro.NewSeq_μ(records).
		Reduce(&totalByCountry, func(acc mapStrInt, r Record) mapStrInt {
			// compute total by country
			acc[r.Country] += int(r.Dates[len(r.Dates)-1].Number)
			// compute total number of case
			totalCases += int(r.Dates[len(r.Dates)-1].Number)
			// find longest country name, used for print formating
			if len(r.Country) > len(longestName) {
				longestName = r.Country
			}
			return acc
		})

	macro.Log_μ(">> Total Number of Cases", totalCases)
	type casesByDate struct {
		date  time.Time
		cases int
	}
	spaces := make([]byte, len(longestName))
	macro.NewSeq_μ(spaces).Map(func(ch byte, i int) byte {
		spaces[i] = ' '
		return spaces[i]
	})

	casesByDates := make([]casesByDate, len(dates))
	var cbd []string
	macro.NewSeq_μ(casesByDates).
		Map(func(v casesByDate, i int) casesByDate {
			// sum all cases by each day
			macro.NewSeq_μ(records).
				Reduce(&casesByDates[i].cases, func(acc int, r Record) int {
					return acc + int(r.Dates[i].Number)
				})
			casesByDates[i].date = dates[i]
			// assign to original slice
			return casesByDates[i]
		}).
		Map(func(v casesByDate, i int) string {
			// convert and format case to string
			bar := make([]byte, int(math.Log2(float64(casesByDates[i].cases))))
			macro.NewSeq_μ(bar).Map(func(ch byte, i int) byte {
				bar[i] = '*'
				return bar[i]
			})
			return fmt.Sprintf("%s %s %d",
				casesByDates[i].date.Format("01/02/06"),
				string(bar), casesByDates[i].cases)
		}).
		Ret(&cbd)

	macro.Log_μ(">> Log Scale")
	macro.PrintSlice_μ(cbd)

	var countries []string
	macro.MapKeys_μ(&countries, totalByCountry)
	sort.Strings(countries)

	macro.Log_μ(">> Sorted by country\n")
	macro.NewSeq_μ(countries).Reduce(&err, func(e error, c string) error {
		fmt.Printf("%s%s : %d\n",
			c, string(spaces[len(c):]), totalByCountry[c])
		return nil
	})

	var css []countryCases
	macro.MapToSlice_μ(&css, totalByCountry,
		func(k string, num int) countryCases { return countryCases{k, num} })

	sort.Slice(css, func(i, j int) bool {
		return css[i].Cases > css[j].Cases
	})

	macro.Log_μ(">> Sorted by number of cases\n")
	macro.NewSeq_μ(css).Reduce(&err, func(e error, c countryCases) error {
		fmt.Printf("%s%s : %d\n",
			c.Country, string(spaces[len(c.Country):]), c.Cases)
		return nil
	})
}

const (
	link       = "https://raw.githubusercontent.com/CSSEGISandData/COVID-19/master/archived_data/time_series/time_series_2019-ncov-Confirmed.csv"
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
