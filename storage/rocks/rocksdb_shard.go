package rocks

import (
	"github.com/dgryski/go-jump"
	"github.com/chrislusf/vasto/storage/codec"
	"time"
)

type shardingCompactionFilter struct {
	shardId    int32
	shardCount int
}

func (m *shardingCompactionFilter) configure(shardId int32, shardCount int) {
	m.shardId = shardId
	m.shardCount = shardCount
}

func (m *shardingCompactionFilter) Name() string { return "vasto.sharding" }
func (m *shardingCompactionFilter) Filter(level int, key, val []byte) (bool, []byte) {
	entry := codec.FromBytes(val)
	jumpHash := jump.Hash(entry.PartitionHash, m.shardCount)
	if m.shardId == jumpHash {
		return false, val
	}
	if entry.UpdatedAtNs/uint64(1000000)+uint64(entry.TtlSecond) < uint64(time.Now().Unix()) {
		return false, val
 	}
	return true, val
}

func (d *Rocks) SetCompactionForShard(shardId, shardCount int) {
	d.compactionFilter.configure(int32(shardId), shardCount)
}
