package frontend

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"time"
)

type void struct{}

type result struct {
	keys map[string]void
	data map[time.Time]map[string]uint64 // timestamps -> keys -> values
}

func newResult() *result {
	return &result{
		keys: make(map[string]void),
		data: make(map[time.Time]map[string]uint64),
	}
}

func (r *result) add(ts time.Time, key string, value uint64) {
	r.keys[key] = void{}
	if _, exists := r.data[ts]; !exists {
		r.data[ts] = make(map[string]uint64)
	}

	r.data[ts][key] = value
}

func (r *result) csv(w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := make([]string, 0)
	header = append(header, "timestamp")
	keys := r.getKeysSorted()
	for _, k := range keys {
		header = append(header, k)
	}

	err := cw.Write(header)
	if err != nil {
		return err
	}

	for _, ts := range r.getTimestampsSorted() {
		record := make([]string, 0)
		record = append(record, ts.Format(time.RFC3339))

		for _, k := range keys {
			record = append(record, fmt.Sprintf("%d", r.data[ts][k]))
		}

		err := cw.Write(record)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *result) getKeysSorted() []string {
	keys := make([]string, len(r.keys))
	i := 0
	for k := range r.keys {
		keys[i] = k
		i++
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	return keys
}

func (r *result) getTimestampsSorted() []time.Time {
	res := make([]time.Time, len(r.data))

	i := 0
	for ts := range r.data {
		res[i] = ts
		i++
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Before(res[j])
	})

	return res
}
