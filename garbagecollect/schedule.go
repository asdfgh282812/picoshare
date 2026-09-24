package garbagecollect

import (
	"log"
	"time"
)

type Scheduler struct {
	collector *Collector
	ticker    *time.Ticker
}

func NewScheduler(collector *Collector, interval time.Duration) Scheduler {
	return Scheduler{
		collector: collector,
		ticker:    time.NewTicker(interval),
	}
}

// StartAsync performs garbage collection immediately and then on every tick.
// Collecting at startup ensures that maintenance still happens when PicoShare
// restarts more often than the collection interval.
func (s *Scheduler) StartAsync() {
	go func() {
		s.collect()
		for range s.ticker.C {
			s.collect()
		}
	}()
}

func (s *Scheduler) collect() {
	log.Printf("performing database maintenance")
	if err := s.collector.Collect(); err != nil {
		log.Printf("database maintenance failed: %v", err)
	}
}
