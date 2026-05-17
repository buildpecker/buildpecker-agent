package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/pthsarmah/forge-agent/internal/system"
	"github.com/pthsarmah/forge-agent/utils"
)

// registerInstruments wires five async gauges to a single callback. The
// PeriodicReader invokes the callback on every flush: it re-reads
// ~/.forge/config.json (via system.GetAllNodes), samples host usage once, and
// emits one datapoint per node_id. Nodes absent this tick are simply not
// observed, so stale series stop without any manual delete.
func registerInstruments(m metric.Meter) error {
	cpu, err := m.Float64ObservableGauge("forge_node_cpu_percent",
		metric.WithDescription("Current host CPU usage percent (0-100)."))
	if err != nil {
		return err
	}
	memUsed, err := m.Int64ObservableGauge("forge_node_memory_used_bytes",
		metric.WithDescription("Resident memory currently in use, in bytes."),
		metric.WithUnit("By"))
	if err != nil {
		return err
	}
	memTotal, err := m.Int64ObservableGauge("forge_node_memory_total_bytes",
		metric.WithDescription("Total physical memory on the host, in bytes."),
		metric.WithUnit("By"))
	if err != nil {
		return err
	}
	diskUsed, err := m.Int64ObservableGauge("forge_node_disk_used_bytes",
		metric.WithDescription("Disk space used on the root filesystem, in bytes."),
		metric.WithUnit("By"))
	if err != nil {
		return err
	}
	diskTotal, err := m.Int64ObservableGauge("forge_node_disk_total_bytes",
		metric.WithDescription("Total disk space on the root filesystem, in bytes."),
		metric.WithUnit("By"))
	if err != nil {
		return err
	}

	_, err = m.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			logger, _ := utils.GetLoggerInstance()

			nodes, err := system.GetAllNodes()
			if err != nil {
				if logger != nil {
					logger.SystemLogger.Printf("metrics: get nodes failed: %v", err)
				}
				return nil
			}

			snap := system.GetSnapshot()
			host := system.GetHostname()

			for _, n := range nodes {
				attrs := metric.WithAttributes(
					attribute.String("node_id", n.NodeId),
					attribute.String("hostname", host),
				)
				o.ObserveFloat64(cpu, snap.CPUPercent, attrs)
				o.ObserveInt64(memUsed, int64(snap.MemUsed), attrs)
				o.ObserveInt64(memTotal, int64(snap.MemTotal), attrs)
				o.ObserveInt64(diskUsed, int64(snap.DiskUsed), attrs)
				o.ObserveInt64(diskTotal, int64(snap.DiskTotal), attrs)
			}
			return nil
		},
		cpu, memUsed, memTotal, diskUsed, diskTotal,
	)
	return err
}
