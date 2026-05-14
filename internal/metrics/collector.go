package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/pthsarmah/forge-agent/internal/system"
	"github.com/pthsarmah/forge-agent/utils"
)

// StartCollector runs a ticker that re-reads ~/.forge/config.json each tick,
// samples host resource usage, and updates one timeseries per node_id.
// Stale node_id series (nodes removed from config) are dropped.
func StartCollector(ctx context.Context, interval time.Duration) {
	logger, _ := utils.GetLoggerInstance()

	hostname := system.GetHostname()

	// tracks which (node_id, hostname) label sets are currently live, so we
	// can DeleteLabelValues for ones that disappear between ticks.
	live := make(map[string]struct{})
	var mu sync.Mutex

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	collect := func() {
		nodes, err := system.GetAllNodes()
		if err != nil {
			logger.SystemLogger.Printf("Metrics collector: get nodes failed: %v", err)
			return
		}

		snap := system.GetSnapshot()

		seen := make(map[string]struct{}, len(nodes))
		for _, n := range nodes {
			key := n.NodeId + "|" + hostname
			seen[key] = struct{}{}

			labels := prometheus.Labels{
				"node_id":  n.NodeId,
				"hostname": hostname,
			}
			cpuPercent.With(labels).Set(snap.CPUPercent)
			memUsed.With(labels).Set(float64(snap.MemUsed))
			memTotal.With(labels).Set(float64(snap.MemTotal))
			diskUsed.With(labels).Set(float64(snap.DiskUsed))
			diskTotal.With(labels).Set(float64(snap.DiskTotal))
		}

		mu.Lock()
		for key := range live {
			if _, still := seen[key]; still {
				continue
			}
			parts := splitKey(key)
			labels := prometheus.Labels{"node_id": parts[0], "hostname": parts[1]}
			cpuPercent.Delete(labels)
			memUsed.Delete(labels)
			memTotal.Delete(labels)
			diskUsed.Delete(labels)
			diskTotal.Delete(labels)
			logger.SystemLogger.Printf("Metrics collector: dropped stale node_id=%s", parts[0])
		}
		live = seen
		mu.Unlock()
	}

	collect()
	logger.SystemLogger.Printf("Metrics collector started interval=%s", interval)

	for {
		select {
		case <-ctx.Done():
			logger.SystemLogger.Println("Metrics collector stopped")
			return
		case <-ticker.C:
			collect()
		}
	}
}

func splitKey(k string) [2]string {
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return [2]string{k[:i], k[i+1:]}
		}
	}
	return [2]string{k, ""}
}
