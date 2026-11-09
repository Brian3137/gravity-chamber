package scouter

import (
	"golang.org/x/sync/singleflight"
)

// Coalescer collapses concurrent identical cache misses into ONE call to the
// underlying loader. The other callers block briefly and share the result.
//
// This is the single highest-value pattern in cache engineering: it turns a
// 1000x database spike into a 1x one, in about twenty lines.
type Coalescer struct {
	group singleflight.Group
}

// Do runs fn for key, sharing the result with any concurrent callers.
func (c *Coalescer) Do(key string, fn func() (any, error)) (any, bool, error) {
	v, err, shared := c.group.Do(key, fn)
	return v, shared, err
}