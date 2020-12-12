package internal

import (
	"expvar"
	"runtime"
	"time"
)

// var (
//   stats *expvar.Map
// )

// func init() {
// 	stats = NewStats("stats")
// }

// TimeVar ...
type TimeVar struct{ v time.Time }

// Set ...
func (o *TimeVar) Set(date time.Time) { o.v = date }

// Add ...
func (o *TimeVar) Add(duration time.Duration) { o.v = o.v.Add(duration) }

// String ...
func (o *TimeVar) String() string { return o.v.Format(time.RFC3339) }

// NewStats ...
func NewStats(name string) *expvar.Map {
	stats := expvar.NewMap(name)

	stats.Set("goroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))
	stats.Set("cgocall", expvar.Func(func() interface{} {
		return runtime.NumCgoCall()
	}))
	stats.Set("cpus", expvar.Func(func() interface{} {
		return runtime.NumCPU()
	}))

	return stats
}
