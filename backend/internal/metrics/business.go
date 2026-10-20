package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "urlshort"

	codeSourceCustom    = "custom"
	codeSourceGenerated = "generated"

	redirectFound    = "found"
	redirectNotFound = "not_found"
)

type Business struct {
	linksCreated  *prometheus.CounterVec
	codeConflicts *prometheus.CounterVec
	linksUpdated  prometheus.Counter
	linksDeleted  prometheus.Counter
	redirects     *prometheus.CounterVec
}

func NewBusiness(reg prometheus.Registerer) *Business {
	factory := promauto.With(reg)

	b := &Business{
		linksCreated: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "links_created_total",
			Help:      "Number of short links created.",
		}, []string{"code_source"}),
		codeConflicts: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "code_conflicts_total",
			Help:      "Number of attempts to use a short code that already exists.",
		}, []string{"code_source"}),
		linksUpdated: factory.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "links_updated_total",
			Help:      "Number of short links updated.",
		}),
		linksDeleted: factory.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "links_deleted_total",
			Help:      "Number of short links deleted.",
		}),
		redirects: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "redirects_total",
			Help:      "Number of short link resolutions by result.",
		}, []string{"result"}),
	}

	for _, source := range []string{codeSourceCustom, codeSourceGenerated} {
		b.linksCreated.WithLabelValues(source)
		b.codeConflicts.WithLabelValues(source)
	}
	for _, result := range []string{redirectFound, redirectNotFound} {
		b.redirects.WithLabelValues(result)
	}

	return b
}

func (b *Business) LinkCreated(customCode bool) {
	b.linksCreated.WithLabelValues(codeSource(customCode)).Inc()
}

func (b *Business) CodeConflict(customCode bool) {
	b.codeConflicts.WithLabelValues(codeSource(customCode)).Inc()
}

func (b *Business) LinkUpdated() {
	b.linksUpdated.Inc()
}

func (b *Business) LinkDeleted() {
	b.linksDeleted.Inc()
}

func (b *Business) Redirect(found bool) {
	result := redirectNotFound
	if found {
		result = redirectFound
	}

	b.redirects.WithLabelValues(result).Inc()
}

func codeSource(customCode bool) string {
	if customCode {
		return codeSourceCustom
	}

	return codeSourceGenerated
}
