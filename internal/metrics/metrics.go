package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	Registry = prometheus.NewRegistry()

	cpuPercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "forge_node_cpu_percent",
			Help: "Current host CPU usage percent (0-100).",
		},
		[]string{"node_id", "hostname"},
	)

	memUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "forge_node_memory_used_bytes",
			Help: "Resident memory currently in use, in bytes.",
		},
		[]string{"node_id", "hostname"},
	)

	memTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "forge_node_memory_total_bytes",
			Help: "Total physical memory on the host, in bytes.",
		},
		[]string{"node_id", "hostname"},
	)

	diskUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "forge_node_disk_used_bytes",
			Help: "Disk space used on the root filesystem, in bytes.",
		},
		[]string{"node_id", "hostname"},
	)

	diskTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "forge_node_disk_total_bytes",
			Help: "Total disk space on the root filesystem, in bytes.",
		},
		[]string{"node_id", "hostname"},
	)
)

func init() {
	Registry.MustRegister(cpuPercent, memUsed, memTotal, diskUsed, diskTotal)
}
