package scouter

import (
	"sort"
	"strconv"

	"github.com/cespare/xxhash/v2"
)

// Ring is a consistent hash ring with virtual nodes.
// Removing a node reassigns only that node's keys (~1/N), instead of the
// ~all-of-them you get with hash(key) % N.
type Ring struct {
	replicas int
	hashes   []uint64          // sorted positions on the ring
	owner    map[uint64]string // position -> node name
}

func NewRing(replicas int, nodes ...string) *Ring {
	r := &Ring{replicas: replicas, owner: make(map[uint64]string)}
	for _, n := range nodes {
		r.Add(n)
	}
	return r
}

func (r *Ring) Add(node string) {
	for i := 0; i < r.replicas; i++ {
		h := xxhash.Sum64String(node + "#" + strconv.Itoa(i))
		r.hashes = append(r.hashes, h)
		r.owner[h] = node
	}
	sort.Slice(r.hashes, func(i, j int) bool { return r.hashes[i] < r.hashes[j] })
}

func (r *Ring) Remove(node string) {
	kept := r.hashes[:0]
	for _, h := range r.hashes {
		if r.owner[h] == node {
			delete(r.owner, h)
			continue
		}
		kept = append(kept, h)
	}
	r.hashes = kept
}

// Get returns the node that owns key: the first ring position clockwise.
func (r *Ring) Get(key string) string {
	if len(r.hashes) == 0 {
		return ""
	}
	h := xxhash.Sum64String(key)
	i := sort.Search(len(r.hashes), func(i int) bool { return r.hashes[i] >= h })
	if i == len(r.hashes) {
		i = 0 // wrap around
	}
	return r.owner[r.hashes[i]]
}

func xxhashString(s string) uint64 { return xxhash.Sum64String(s) }