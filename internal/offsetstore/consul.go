package offsetstore

import (
	"fmt"
	"strconv"
	"time"

	"cwsync/internal/logger"

	"github.com/hashicorp/consul/api"
)

type ConsulOffsetStore struct{ Client *api.Client }

func (c *ConsulOffsetStore) SaveOffset(kvPath string, lastTimestamp int64) error {
	if c.Client == nil {
		return fmt.Errorf("consul client not initialized")
	}
	kvPair := &api.KVPair{Key: kvPath, Value: []byte(fmt.Sprintf("%d", lastTimestamp))}
	_, err := c.Client.KV().Put(kvPair, nil)
	return err
}

func (c *ConsulOffsetStore) LoadOffset(kvPath string, fallbackDuration time.Duration) int64 {
	if c.Client == nil {
		logger.Fatal.Fatalf("consul client not initialized")
	}
	kvPair, _, err := c.Client.KV().Get(kvPath, nil)
	if err != nil {
		logger.Fatal.Fatalf("Failed to load offset from Consul: %v", err)
	}
	if kvPair == nil {
		logger.Info.Printf("Offset not found in Consul, using default timestamp of %s", fallbackDuration)
		return time.Now().UTC().Add(-fallbackDuration).UnixMilli()
	}
	lastTimestamp, err := strconv.ParseInt(string(kvPair.Value), 10, 64)
	if err != nil {
		logger.Fatal.Fatalf("Failed to parse offset from Consul: %v", err)
	}
	return lastTimestamp
}
