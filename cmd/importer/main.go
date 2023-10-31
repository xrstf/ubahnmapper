package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/spf13/pflag"
)

var (
	timeShift    = pflag.DurationP("time-shift", "s", 0, "shift start of timeseries by this much time (Go duration, e.g. '30m')")
	protocolFile = pflag.StringP("protocol", "p", "", "protocol CSV file")
	runID        = pflag.StringP("run-id", "i", "", "unique identifier for this timeseries")
	timezone     = pflag.StringP("timezone", "t", "Europe/Berlin", "timezone to interpret the timestamps with")
	basePressure = pflag.Float64P("base-pressure", "b", 0, "instead of taking the first datapoint as the base pressure, use this value")
	eventRange   = pflag.BoolP("event-range", "r", false, "trim any data points before the first and after the last event")
)

type Datapoint struct {
	Recorded time.Time
	Pressure float64
	Event    string
}

type Timeseries struct {
	Points []Datapoint

	TimeOffset     time.Duration
	PressureOffset float64
}

func main() {
	pflag.Parse()

	if pflag.NArg() == 0 {
		log.Fatal("No data file given.")
	}

	if *runID == "" {
		log.Fatal("No --run-id given.")
	}

	loc, err := time.LoadLocation(*timezone)
	if err != nil {
		log.Fatalf("Invalid timezone %q: %v", *timezone, err)
	}

	dataTimeseries, err := loadData(pflag.Arg(0), loc)
	if err != nil {
		log.Fatalf("Failed to load data file: %v", err)
	}

	if *protocolFile != "" {
		protocolTimeseries, err := loadProtocol(*protocolFile, loc)
		if err != nil {
			log.Fatalf("Failed to load protocol file: %v", err)
		}

		combinedTimeseries, err := combineTimeseries(dataTimeseries, protocolTimeseries)
		if err != nil {
			log.Fatalf("Failed to combine timeseries: %v", err)
		}

		dataTimeseries = combinedTimeseries
	}

	if *eventRange {
		dataTimeseries, err = trimTimeseries(dataTimeseries)
		if err != nil {
			log.Fatalf("Failed to trim timeseries: %v", err)
		}
	}

	dataTimeseries, err = normalizeTimeseries(dataTimeseries, timeShift, *basePressure)
	if err != nil {
		log.Fatalf("Failed to normalize timeseries: %v", err)
	}

	printTimeseriesSQL(dataTimeseries, pflag.Arg(0), *runID)
}

func loadData(filename string, tz *time.Location) (*Timeseries, error) {
	ts := &Timeseries{
		Points: []Datapoint{},
	}

	if err := loadCSV(filename, tz, 1, func(t time.Time, data []string) error {
		p, err := strconv.ParseFloat(data[0], 32)
		if err != nil {
			return fmt.Errorf("invalid pressure value %q: %w", data[0], err)
		}

		ts.Points = append(ts.Points, Datapoint{
			Recorded: t,
			Pressure: p,
		})

		return nil
	}); err != nil {
		return nil, err
	}

	return ts, nil
}

func loadProtocol(filename string, tz *time.Location) (*Timeseries, error) {
	ts := &Timeseries{
		Points: []Datapoint{},
	}

	if err := loadCSV(filename, tz, 0, func(t time.Time, data []string) error {
		ts.Points = append(ts.Points, Datapoint{
			Recorded: t,
			Event:    data[0],
		})

		return nil
	}); err != nil {
		return nil, err
	}

	return ts, nil
}

func loadCSV(filename string, tz *time.Location, headerRows int, handler func(t time.Time, data []string) error) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	csvReader.Comma = ';'
	csvReader.ReuseRecord = true

	for i := 0; i < headerRows; i++ {
		_, err = csvReader.Read()
		if err != nil {
			return err
		}
	}

	for {
		record, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		if len(record) < 2 {
			return fmt.Errorf("invalid record: %v", record)
		}

		t, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", record[0], tz)
		if err != nil {
			return fmt.Errorf("invalid timestamp %q: %w", record[0], err)
		}

		if err := handler(t, record[1:]); err != nil {
			return err
		}
	}

	return nil
}

func combineTimeseries(data, protocol *Timeseries) (*Timeseries, error) {
	// make a copy of the original data
	result := &Timeseries{
		Points: append([]Datapoint{}, data.Points...),
	}

	// fill in the event names
	for _, e := range protocol.Points {
		for i, dataPoint := range result.Points {
			if dataPoint.Recorded.After(e.Recorded) {
				result.Points[i].Event = e.Event
				break
			}
		}
	}

	return result, nil
}

func normalizeTimeseries(data *Timeseries, timeShift *time.Duration, basePressure float64) (*Timeseries, error) {
	// make a copy of the original data
	result := &Timeseries{
		Points: append([]Datapoint{}, data.Points...),
	}

	if len(data.Points) == 0 {
		return result, nil
	}

	startTime := data.Points[0].Recorded
	roundedStartTime := startTime.Round(time.Hour)
	offset := roundedStartTime.Sub(startTime)

	if timeShift != nil {
		offset = offset + *timeShift
	}

	if basePressure == 0 {
		basePressure = data.Points[0].Pressure
	}

	result.TimeOffset = offset
	result.PressureOffset = basePressure

	for i, dataPoint := range result.Points {
		result.Points[i].Recorded = dataPoint.Recorded.Add(offset)
		result.Points[i].Pressure = dataPoint.Pressure - basePressure
	}

	return result, nil
}

func trimTimeseries(data *Timeseries) (*Timeseries, error) {
	firstEvent := -1
	for i, p := range data.Points {
		if p.Event != "" {
			if firstEvent == -1 {
				firstEvent = i
			}
		}
	}

	if firstEvent != -1 {
		data.Points = data.Points[firstEvent:]
	}

	lastEvent := -1
	for i, p := range data.Points {
		if p.Event != "" {
			lastEvent = i
		}
	}

	if lastEvent != -1 {
		data.Points = data.Points[:lastEvent+1]
	}

	return data, nil
}

func printTimeseriesSQL(data *Timeseries, filename string, runID string) {
	fmt.Printf("-- input file.....: %v\n", filename)
	fmt.Printf("-- time offset....: %v\n", data.TimeOffset)
	fmt.Printf("-- pressure offset: %v hPa\n", data.PressureOffset)
	fmt.Println("")

	for _, p := range data.Points {
		comment := "NULL"
		if p.Event != "" {
			comment = fmt.Sprintf(`'%s'`, p.Event)
		}

		fmt.Printf(`INSERT INTO ubahnmapper ("time", "run_id", "pressure", "comment") VALUES ('%s', '%s', %v, %s);`, p.Recorded.Format("2006-01-02T15:04:05.999999999"), runID, p.Pressure, comment)
		fmt.Println("")
	}
}
