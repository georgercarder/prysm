package blockchain

import (
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
)

var (
	// maxCacheSize defines the max number of committee info this can cache.
	// Due to reorgs and long finality, it's good to keep the old cache around for quickly switch over.
	maxCacheSize = 32

	// cacheMiss tracks the number of check point info  requests that aren't present in the cache.
	cacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "check_point_info_cache_miss",
		Help: "The number of check point info requests that aren't present in the cache.",
	})
	// cacheHit tracks the number of check point info  requests that are in the cache.
	cacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "check_point_info_cache_hit",
		Help: "The number of check point info requests that are present in the cache.",
	})
)

// CheckPtInfo defines struct with necessary fields to verify an attestation signature.
type CheckPtInfo struct {
	fork          *pb.Fork
	genesisRoot   [32]byte
	seed          [32]byte
	activeIndices []uint64
	pubKeys       [][48]byte
}

// checkPtInfoCache is a struct with 1 queue for looking up check point info by checkpoint.
type checkPtInfoCache struct {
	cache *lru.Cache
	lock  sync.RWMutex
}

// newCheckPointInfoCache creates a new checkpoint state cache for storing/accessing processed state.
func newCheckPointInfoCache() *checkPtInfoCache {
	cache, err := lru.New(maxCacheSize)
	if err != nil {
		panic(err)
	}
	return &checkPtInfoCache{
		cache: cache,
	}
}

// get fetches info by checkpoint. Returns the reference of the CheckPtInfo, nil if doesn't exist.
func (c *checkPtInfoCache) get(cp *ethpb.Checkpoint) (*CheckPtInfo, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	h, err := hashutil.HashProto(cp)
	if err != nil {
		return nil, err
	}

	item, exists := c.cache.Get(h)

	if exists && item != nil {
		cacheHit.Inc()
		// Copy here is unnecessary since the return will only be used to verify attestation signature.
		return item.(*CheckPtInfo), nil
	}

	cacheMiss.Inc()
	return nil, nil
}

// put adds CheckPtInfo info object to the cache. This method also trims the least
// recently added CheckPtInfo object if the cache size has ready the max cache size limit.
func (c *checkPtInfoCache) put(cp *ethpb.Checkpoint, f *pb.Fork, g [32]byte, s [32]byte, indices []uint64, pk [][48]byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	h, err := hashutil.HashProto(cp)
	if err != nil {
		return err
	}

	info := &CheckPtInfo{
		fork:          f,
		genesisRoot:   g,
		seed:          s,
		activeIndices: indices,
		pubKeys:       pk,
	}

	c.cache.Add(h, info)
	return nil
}

func (c *CheckPtInfo) Fork() *pb.Fork {
	return c.fork
}

func (c *CheckPtInfo) GenesisRoot() [32]byte {
	return c.genesisRoot
}

func (c *CheckPtInfo) Seed() [32]byte {
	return c.seed
}

func (c *CheckPtInfo) ActiveIndices() []uint64 {
	return c.activeIndices
}

func (c *CheckPtInfo) ActiveCount() uint64 {
	return uint64(len(c.activeIndices))
}

func (c *CheckPtInfo) Pubkey(i uint64) [48]byte {
	return c.pubKeys[i]
}

func (c *CheckPtInfo) Pubkeys() [][48]byte {
	return c.pubKeys
}
