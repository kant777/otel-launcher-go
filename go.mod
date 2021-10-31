module github.com/kant777/otel-launcher-go

go 1.16

require (
	github.com/kant777/otel-launcher-go/pipelines v0.0.0
	github.com/sethvargo/go-envconfig v0.3.2
	go.opentelemetry.io/collector v0.28.0
	go.opentelemetry.io/otel v1.0.0
	go.opentelemetry.io/otel/metric v0.23.0
	go.opentelemetry.io/otel/sdk v1.0.0
	go.opentelemetry.io/otel/trace v1.0.0
)

replace github.com/kant777/otel-launcher-go/pipelines => ./pipelines
