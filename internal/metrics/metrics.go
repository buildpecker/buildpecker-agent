package metrics

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/pthsarmah/forge-agent/internal/system"
	"github.com/pthsarmah/forge-agent/utils"
)

// defaultEndpoint is the OTel Collector OTLP/HTTP receiver in the central
// backend. host:port only — exporter appends /v1/metrics.
const defaultEndpoint = "localhost:4318"

// Start sets up an OTLP/HTTP metrics push pipeline: a PeriodicReader flushes
// observable gauges to the central OTel Collector every interval. Returns a
// shutdown func that flushes once more and tears the provider down.
func Start(ctx context.Context, endpoint string, interval time.Duration) (func(context.Context) error, error) {
	logger, _ := utils.GetLoggerInstance()

	if endpoint == "" {
		if env := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); env != "" {
			endpoint = env
		} else {
			endpoint = defaultEndpoint
		}
	}

	exp, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "forge-agent"),
			attribute.String("host.name", system.GetHostname()),
		),
	)
	if err != nil {
		return nil, err
	}

	reader := sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(interval))
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(mp)

	if err := registerInstruments(mp.Meter("forge-agent")); err != nil {
		_ = mp.Shutdown(ctx)
		return nil, err
	}

	logger.SystemLogger.Printf("OTLP metrics push started endpoint=%s interval=%s", endpoint, interval)
	return mp.Shutdown, nil
}
