// Package shard routes requests to backends by hashing a key, so the same
// key always lands on the same backend ("sticky routing"). This is the
// classic trick for keeping per-user sessions, caches, or data local to
// one machine. It mirrors the sharding idea from the data-management POC
// (siamo-poc-data-management), applied to request routing instead of
// storage.
package shard

import (
	"hash/fnv"
	"sort"
	"strconv"
)

// Ring is a consistent-hash ring with virtual nodes. Adding or removing a
// backend only remaps ~1/N of keys instead of reshuffling everything.
type Ring struct {
	keys    []uint32          // sorted hash points
	members map[uint32]string // hash point -> backend id
}

// New builds a ring over the given backend ids, each with vnodes virtual
// nodes for even spread.
func New(ids []string, vnodes int) *Ring {
	r := &Ring{members: map[uint32]string{}}
	for _, id := range ids {
		for i := 0; i < vnodes; i++ {
			h := hash(id + "#" + strconv.Itoa(i))
			r.keys = append(r.keys, h)
			r.members[h] = id
		}
	}
	sort.Slice(r.keys, func(i, j int) bool { return r.keys[i] < r.keys[j] })
	return r
}

// Route returns the backend id that owns key.
func (r *Ring) Route(key string) string {
	if len(r.keys) == 0 {
		return ""
	}
	h := hash(key)
	// First hash point at-or-after h; wrap around the ring.
	i := sort.Search(len(r.keys), func(i int) bool { return r.keys[i] >= h })
	if i == len(r.keys) {
		i = 0
	}
	return r.members[r.keys[i]]
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
