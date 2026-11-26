package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the service counters on its own registry, so nothing is global and a second instance cannot clash with the first.
type Metrics struct {
	reg              *prometheus.Registry
	redirects        *prometheus.CounterVec
	redirectDuration prometheus.Histogram
	creates          *prometheus.CounterVec
	aliasConflicts   prometheus.Counter
	deletes          prometheus.Counter
	cacheErrors      prometheus.Counter
	rangeRefills     prometheus.Counter
	rangeRefillErrs  prometheus.Counter
}

// Label values; they match the constants in the redirect and shorten packages.
var (
	redirectResults = []string{"cache_hit", "store_hit", "not_found", "expired", "deleted"}
	createKinds     = []string{"generated", "alias"}
)

// New returns Metrics with every series registered, so a counter that has not fired yet still shows as 0.
func New() *Metrics {
	m := &Metrics{reg: prometheus.NewRegistry()}
	m.redirects = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tinyurl_redirects_total", Help: "Redirect lookups by result."}, []string{"result"})
	m.creates = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tinyurl_creates_total", Help: "Links created by kind."}, []string{"kind"})
	m.redirectDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "tinyurl_redirect_duration_seconds", Help: "Redirect request duration.", Buckets: prometheus.DefBuckets,
	})
	m.aliasConflicts = newCounter("tinyurl_alias_conflicts_total", "Creates refused because the alias was taken.")
	m.deletes = newCounter("tinyurl_deletes_total", "Links deleted.")
	m.cacheErrors = newCounter("tinyurl_cache_errors_total", "Cache calls that failed.")
	m.rangeRefills = newCounter("tinyurl_range_refills_total", "Id ranges claimed from the allocator.")
	m.rangeRefillErrs = newCounter("tinyurl_range_refill_errors_total", "Id range claims that failed.")
	m.reg.MustRegister(m.redirects, m.creates, m.redirectDuration, m.aliasConflicts, m.deletes, m.cacheErrors, m.rangeRefills, m.rangeRefillErrs)
	for _, r := range redirectResults {
		m.redirects.WithLabelValues(r)
	}
	for _, k := range createKinds {
		m.creates.WithLabelValues(k)
	}
	return m
}

func newCounter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{Name: name, Help: help})
}

// Handler serves the metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler { return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}) }

// Resolved counts a redirect lookup by result.
func (m *Metrics) Resolved(result string) { m.redirects.WithLabelValues(result).Inc() }

// RedirectDuration records how long a redirect request took.
func (m *Metrics) RedirectDuration(d time.Duration) { m.redirectDuration.Observe(d.Seconds()) }

// Created counts a new link by kind.
func (m *Metrics) Created(kind string) { m.creates.WithLabelValues(kind).Inc() }

// AliasConflict counts a create refused because the alias was taken.
func (m *Metrics) AliasConflict() { m.aliasConflicts.Inc() }

// Deleted counts a deleted link.
func (m *Metrics) Deleted() { m.deletes.Inc() }

// CacheError counts a failed cache call.
func (m *Metrics) CacheError() { m.cacheErrors.Inc() }

// RangeRefilled counts an id range claimed from the allocator.
func (m *Metrics) RangeRefilled() { m.rangeRefills.Inc() }

// RangeRefillFailed counts an id range claim that failed.
func (m *Metrics) RangeRefillFailed() { m.rangeRefillErrs.Inc() }
