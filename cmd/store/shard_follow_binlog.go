package store

import (
	"fmt"
	"log"
	"time"

	"github.com/chrislusf/vasto/pb"
	"github.com/chrislusf/vasto/storage/codec"
	"github.com/chrislusf/vasto/topology"
	"google.golang.org/grpc"
	"context"
)

const (
	syncProgressFlushInterval = time.Minute
)

func (s *shard) followChanges(ctx context.Context, node topology.Node, grpcConnection *grpc.ClientConn, targetClusterSize uint32, targetShardId int) (error) {

	client := pb.NewVastoStoreClient(grpcConnection)

	nextSegment, nextOffset, _, err := s.loadProgress(node.GetAdminAddress())
	if err != nil {
		log.Printf("read shard %d follow progress: %v", s.id, err)
	}

	// set in memory progress
	s.insertInMemoryFollowProgress(node.GetAdminAddress(), nextSegment, nextOffset)

	log.Printf("shard %v follows server %d from segment %d offset %d", s.String(), node.GetId(), nextSegment, nextOffset)

	request := &pb.PullUpdateRequest{
		Keyspace:          s.keyspace,
		ShardId:           uint32(s.id),
		Segment:           nextSegment,
		Offset:            nextOffset,
		Limit:             8096,
		TargetClusterSize: targetClusterSize,
		TargetShardId:     uint32(targetShardId),
		Origin:            s.String(),
	}

	stream, err := client.TailBinlog(ctx, request)
	if err != nil {
		return fmt.Errorf("client.TailBinlog to server %d %s: %v", node.GetId(), node.GetAdminAddress(), err)
	}

	for {

		// println("TailBinlog receive from", s.id)

		changes, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("pull changes: %v", err)
		}

		// log.Printf("%s follow 0 entry: %d", s, len(changes.Entries))

		for _, entry := range changes.Entries {

			// log.Printf("%s follow 1 entry: %v", s, string(entry.Key))

			b, err := s.db.Get(entry.Key)
			if err != nil {
				continue
			}

			// process deletes
			if entry.IsDelete {
				if err == nil && len(b) > 0 {
					row := codec.FromBytes(b)
					if row.IsExpired() {
						continue
					}
					if row.UpdatedAtNs > entry.UpdatedAtNs {
						continue
					}
					s.db.Delete(entry.Key)
				}
				continue
			}

			// log.Printf("%s follow 2 entry: %v, err: %v, len(b)=%d", s, string(entry.Key), err, len(b))

			// process put
			t := &codec.Entry{
				PartitionHash: entry.PartitionHash,
				UpdatedAtNs:   entry.UpdatedAtNs,
				TtlSecond:     entry.TtlSecond,
				Value:         entry.Value,
			}
			if len(b) == 0 {
				// no existing data found
				s.db.Put(entry.Key, t.ToBytes())
				continue
			}

			row := codec.FromBytes(b)
			// log.Printf("%s follow 3 entry: %v, expired %v, %v", s, string(entry.Key), row.IsExpired(), t.IsExpired())
			if row.IsExpired() {
				if !t.IsExpired() {
					// log.Printf("%s follow 3 entry: %v", s, string(entry.Key))
					s.db.Put(entry.Key, t.ToBytes())
					continue
				}
			} else {
				// log.Printf("data time %d, existing time %d, delta %d", row.UpdatedAtNs, entry.UpdatedAtNs, row.UpdatedAtNs-entry.UpdatedAtNs)
				if row.UpdatedAtNs > entry.UpdatedAtNs {
					continue
				}
				s.db.Put(entry.Key, t.ToBytes())
				continue
			}
			// log.Printf("%s follow 4 entry: %v", s, string(entry.Key))

		}

		// set the nextSegment and nextOffset
		nextSegment, nextOffset = changes.NextSegment, changes.NextOffset
		if !s.updateInMemoryFollowProgressIfPresent(node.GetAdminAddress(), nextSegment, nextOffset) {
			// if not found in memory,
			break
		}

	}

	return nil
}
