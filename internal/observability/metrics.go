package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	EmbeddingsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "embeddings_total",
			Help: "Total de embeddings gerados",
		},
	)
)

func Start(port string) {
	prometheus.MustRegister(EmbeddingsTotal)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":"+port, nil)
}
