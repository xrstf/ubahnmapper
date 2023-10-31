package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var (
	timeShift     = pflag.DurationP("time-shift", "s", 0, "shift start of timeseries by this much time (Go duration, e.g. '30m')")
	collapseStops = pflag.DurationP("collapse-stops", "c", 0, "collapse all data points between ' an' and ' ab' events (Go duration, e.g. '30m') (requires --protocol)")
	protocolFile  = pflag.StringP("protocol", "p", "", "protocol CSV file")
	runID         = pflag.StringP("run-id", "i", "", "unique identifier for this timeseries")
	timezone      = pflag.StringP("timezone", "t", "Europe/Berlin", "timezone to interpret the timestamps with")
	basePressure  = pflag.Float64P("base-pressure", "b", 0, "instead of taking the first datapoint as the base pressure, use this value")
	eventRange    = pflag.BoolP("event-range", "r", false, "trim any data points before the first and after the last event")
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

	if *collapseStops > 0 {
		dataTimeseries, err = collapseStopsInTimeseries(dataTimeseries, *collapseStops)
		if err != nil {
			log.Fatalf("Failed to collapse stops in timeseries: %v", err)
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

func collapseStopsInTimeseries(data *Timeseries, collapseDuration time.Duration) (*Timeseries, error) {
	if len(data.Points) < 2 {
		return data, nil
	}

	result := &Timeseries{
		Points: []Datapoint{},
	}

	totalPoints := len(data.Points)

	// every time we replace a chunk of points with a fixed length stop,
	// we have to shift all following points and keep updating the shift
	// based on each stop
	var timeShift time.Duration

	for i := 0; i < totalPoints; i++ {
		point := data.Points[i]

		if isArrival(point) {
			departure := -1
			pressures := []float64{point.Pressure}

			// collect all following data points until we find the departure
			for j := i + 1; j < totalPoints; j++ {
				jPoint := data.Points[j]

				if isArrival(jPoint) {
					return nil, fmt.Errorf("arrival follows arrival, missing departure event for arrival @ %v", point.Recorded)
				}

				pressures = append(pressures, jPoint.Pressure)

				if isDeparture(jPoint) {
					departure = j
					break
				}
			}

			// if we found a departure point, great :)
			if departure >= 0 {
				// calculate the average pressure during the stop
				pressure := average(pressures)

				departurePoint := data.Points[departure]

				// add 2 points to the result, so they form a straight
				// line between them with the defined length (duration)
				result.Points = append(result.Points, Datapoint{
					Recorded: point.Recorded.Add(timeShift),
					Pressure: pressure,
					Event:    point.Event,
				}, Datapoint{
					Recorded: point.Recorded.Add(timeShift).Add(collapseDuration),
					Pressure: pressure,
					Event:    departurePoint.Event,
				})

				// calculate how much to shift all following points based
				// on this change; first calculate how long the real-life
				// stop actually was
				actualDuration := departurePoint.Recorded.Sub(point.Recorded)
				newShift := collapseDuration - actualDuration

				// add this shift to all the others
				timeShift += newShift

				// continue scanning after the departure element
				i = departure
			} else {
				// if no departure was found, add the arrival and continue normally
				result.Points = append(result.Points, Datapoint{
					Recorded: point.Recorded.Add(timeShift),
					Pressure: point.Pressure,
					Event:    point.Event,
				})
			}

			continue
		}

		result.Points = append(result.Points, Datapoint{
			Recorded: point.Recorded.Add(timeShift),
			Pressure: point.Pressure,
			Event:    point.Event,
		})
	}

	return result, nil
}

func average(values []float64) float64 {
	acc := float64(0)
	for _, val := range values {
		acc += val
	}

	return acc / float64(len(values))
}

func isArrival(p Datapoint) bool {
	return strings.HasSuffix(p.Event, " an")
}

func isDeparture(p Datapoint) bool {
	return strings.HasSuffix(p.Event, " ab")
}

func printTimeseriesSQL(data *Timeseries, filename string, runID string) {
	fmt.Printf("-- input file.....: %v\n", filename)
	fmt.Printf("-- time offset....: %v\n", data.TimeOffset)
	fmt.Printf("-- pressure offset: %v hPa\n", data.PressureOffset)
	fmt.Println("")
	fmt.Println("BEGIN;")

	for _, p := range data.Points {
		comment := "NULL"
		if p.Event != "" {
			comment = fmt.Sprintf(`'%s'`, p.Event)
		}

		fmt.Printf(`INSERT INTO ubahnmapper ("time", "run_id", "pressure", "comment") VALUES ('%s', '%s', %v, %s);`, p.Recorded.Format("2006-01-02T15:04:05.999999999"), runID, p.Pressure, comment)
		fmt.Println("")
	}

	fmt.Println("COMMIT;")
}
