package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Track total confirmed bookings
	ConfirmedBookings = promauto.NewCounter(prometheus.CounterOpts{
		Name: "atlastickets_bookings_confirmed_total",
		Help: "The total number of successfully confirmed ticket bookings",
	})

	// Track sold out events
	SoldOutEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "atlastickets_bookings_sold_out_total",
		Help: "The total number of rejected bookings due to no inventory",
	})

	// Track active workers in the pool
	ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "atlastickets_active_workers",
		Help: "Current number of active workers in the Go worker pool",
	})
)
