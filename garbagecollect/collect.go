package garbagecollect

import (
	"sync"
	"time"
)

type (
	DatabasePurger interface {
		Purge() error
	}

	// CollectionResult describes a single garbage collection run. The zero
	// value means that no collection has run yet.
	CollectionResult struct {
		Time time.Time
		Err  error
	}

	Collector struct {
		purger  DatabasePurger
		now     func() time.Time
		mu      sync.Mutex
		lastRun CollectionResult
	}
)

func NewCollector(purger DatabasePurger, now func() time.Time) Collector {
	return Collector{
		purger: purger,
		now:    now,
	}
}

func (c *Collector) Collect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.purger.Purge()
	c.lastRun = CollectionResult{
		Time: c.now(),
		Err:  err,
	}

	return err
}

// LastRun returns the result of the most recent garbage collection.
func (c *Collector) LastRun() CollectionResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.lastRun
}
