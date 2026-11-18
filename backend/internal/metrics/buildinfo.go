package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func NewBuildInfo(reg prometheus.Registerer, version string) {
	promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "build_info",
		Help:        "Build version of the running binary; the value is always 1.",
		ConstLabels: prometheus.Labels{"version": version},
	}).Set(1)
}
