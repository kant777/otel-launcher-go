// Copyright Lightstep Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kant777/otel-launcher-go/pipelines"
	"github.com/sethvargo/go-envconfig"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

type Option func(*Config)

// WithAccessToken configures the lightstep access token
// remain compatible with the Lightstep-only launcher for now...
func WithAccessToken(accessToken string) Option {
	// this will actually get used
	return func(c *Config) {
		// duplicating this isn't great but for now let's just get it working
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		c.Headers["lightstep-access-token"] = accessToken
	}
}

// WithMetricExporterEndpoint configures the endpoint for sending metrics via OTLP
func WithMetricExporterEndpoint(url string) Option {
	return func(c *Config) {
		c.MetricExporterEndpoint = url
	}
}

// WithSpanExporterEndpoint configures the endpoint for sending traces via OTLP
func WithSpanExporterEndpoint(url string) Option {
	return func(c *Config) {
		c.SpanExporterEndpoint = url
	}
}

// WithServiceName configures a "service.name" resource label
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.ServiceName = name
	}
}

// WithServiceVersion configures a "service.version" resource label
func WithServiceVersion(version string) Option {
	return func(c *Config) {
		c.ServiceVersion = version
	}
}

// WithHeaders configures OTLP/gRPC connection headers
func WithHeaders(headers map[string]string) Option {
	return func(c *Config) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		for k, v := range headers {
			c.Headers[k] = v
		}
	}
}

// WithLogLevel configures the logging level for OpenTelemetry
func WithLogLevel(loglevel string) Option {
	return func(c *Config) {
		c.LogLevel = loglevel
	}
}

// WithSpanExporterInsecure permits connecting to the
// trace endpoint without a certificate
func WithSpanExporterInsecure(insecure bool) Option {
	return func(c *Config) {
		c.SpanExporterEndpointInsecure = insecure
	}
}

// WithMetricExporterInsecure permits connecting to the
// metric endpoint without a certificate
func WithMetricExporterInsecure(insecure bool) Option {
	return func(c *Config) {
		c.MetricExporterEndpointInsecure = insecure
	}
}

// WithResourceAttributes configures attributes on the resource
func WithResourceAttributes(attributes map[string]string) Option {
	return func(c *Config) {
		c.resourceAttributes = attributes
	}
}

// WithPropagators configures propagators
func WithPropagators(propagators []string) Option {
	return func(c *Config) {
		c.Propagators = propagators
	}
}

// Configures a global error handler to be used throughout an OpenTelemetry instrumented project.
// See "go.opentelemetry.io/otel"
func WithErrorHandler(handler otel.ErrorHandler) Option {
	return func(c *Config) {
		c.errorHandler = handler
	}
}

// WithMetricReportingPeriod configures the metric reporting period,
// how often the controller collects and exports metric data.
func WithMetricReportingPeriod(p time.Duration) Option {
	return func(c *Config) {
		c.MetricReportingPeriod = fmt.Sprint(p)
	}
}

// WithMetricEnabled configures whether metrics should be enabled
func WithMetricsEnabled(enabled bool) Option {
	return func(c *Config) {
		c.MetricsEnabled = enabled
	}
}

type Logger interface {
	Fatalf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}

func WithLogger(logger Logger) Option {
	return func(c *Config) {
		c.logger = logger
	}
}

type DefaultLogger struct {
}

func (l *DefaultLogger) Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}

func (l *DefaultLogger) Debugf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

type defaultHandler struct {
	logger Logger
}

func (l *defaultHandler) Handle(err error) {
	l.logger.Debugf("error: %v\n", err)
}

type Config struct {
	SpanExporterEndpoint           string            `env:"OTEL_EXPORTER_OTLP_SPAN_ENDPOINT"`
	SpanExporterEndpointInsecure   bool              `env:"OTEL_EXPORTER_OTLP_SPAN_INSECURE,default=false"`
	ServiceName                    string            `env:"SERVICE_NAME"`
	ServiceVersion                 string            `env:"SERVICE_VERSION,default=unknown"`
	Headers                        map[string]string `env:"OTEL_EXPORTER_OTLP_HEADERS"`
	MetricExporterEndpoint         string            `env:"OTEL_EXPORTER_OTLP_METRIC_ENDPOINT"`
	MetricExporterEndpointInsecure bool              `env:"OTEL_EXPORTER_OTLP_METRIC_INSECURE,default=false"`
	MetricsEnabled                 bool              `env:"METRICS_ENABLED,default=true"`
	LogLevel                       string            `env:"OTEL_LOG_LEVEL,default=info"`
	Propagators                    []string          `env:"OTEL_PROPAGATORS,default=b3"`
	MetricReportingPeriod          string            `env:"OTEL_EXPORTER_OTLP_METRIC_PERIOD,default=30s"`
	resourceAttributes             map[string]string
	Resource                       *resource.Resource
	logger                         Logger
	errorHandler                   otel.ErrorHandler
}

func validateConfiguration(c Config) error {
	if len(c.ServiceName) == 0 {
		serviceNameSet := false
		for _, kv := range c.Resource.Attributes() {
			if kv.Key == conventions.AttributeServiceName {
				if len(kv.Value.AsString()) > 0 {
					serviceNameSet = true
				}
				break
			}
		}
		if !serviceNameSet {
			return errors.New("invalid configuration: service name missing. Set SERVICE_NAME env var or configure WithServiceName in code")
		}
	}

	return nil
}

func newConfig(opts ...Option) Config {
	var c Config
	envError := envconfig.Process(context.Background(), &c)
	c.logger = &DefaultLogger{}
	c.errorHandler = &defaultHandler{logger: c.logger}
	var defaultOpts []Option

	for _, opt := range append(defaultOpts, opts...) {
		opt(&c)
	}
	c.Resource = newResource(&c)

	if envError != nil {
		c.logger.Fatalf("environment error: %v", envError)
	}

	return c
}

type Launcher struct {
	config        Config
	shutdownFuncs []func() error
}

func newResource(c *Config) *resource.Resource {
	r := resource.Environment()

	hostnameSet := false
	for iter := r.Iter(); iter.Next(); {
		if iter.Attribute().Key == conventions.AttributeHostName && len(iter.Attribute().Value.Emit()) > 0 {
			hostnameSet = true
		}
	}

	attributes := []attribute.KeyValue{
		attribute.String(conventions.AttributeTelemetrySDKName, "launcher"),
		attribute.String(conventions.AttributeTelemetrySDKLanguage, "go"),
		attribute.String(conventions.AttributeTelemetrySDKVersion, version),
	}

	if len(c.ServiceName) > 0 {
		attributes = append(attributes, attribute.String(conventions.AttributeServiceName, c.ServiceName))
	}

	if len(c.ServiceVersion) > 0 {
		attributes = append(attributes, attribute.String(conventions.AttributeServiceVersion, c.ServiceVersion))
	}

	for key, value := range c.resourceAttributes {
		if len(value) > 0 {
			if key == conventions.AttributeHostName {
				hostnameSet = true
			}
			attributes = append(attributes, attribute.String(key, value))
		}
	}

	if !hostnameSet {
		hostname, err := os.Hostname()
		if err != nil {
			c.logger.Debugf("unable to set host.name. Set OTEL_RESOURCE_ATTRIBUTES=\"host.name=<your_host_name>\" env var or configure WithResourceAttributes in code: %v", err)
		} else {
			attributes = append(attributes, attribute.String(conventions.AttributeHostName, hostname))
		}
	}

	attributes = append(r.Attributes(), attributes...)

	// These detectors can't actually fail, ignoring the error.
	r, _ = resource.New(
		context.Background(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(attributes...),
	)

	// Note: There are new detectors we may wish to take advantage
	// of, now available in the default SDK (e.g., WithProcess(),
	// WithOSType(), ...).
	return r
}

func setupTracing(c Config) (func() error, error) {
	if c.SpanExporterEndpoint == "" {
		c.logger.Debugf("tracing is disabled by configuration: no endpoint set")
		return nil, nil
	}
	return pipelines.NewTracePipeline(pipelines.PipelineConfig{
		Endpoint:    c.SpanExporterEndpoint,
		Insecure:    c.SpanExporterEndpointInsecure,
		Headers:     c.Headers,
		Resource:    c.Resource,
		Propagators: c.Propagators,
	})
}

type setupFunc func(Config) (func() error, error)

func setupMetrics(c Config) (func() error, error) {
	if !c.MetricsEnabled {
		c.logger.Debugf("metrics are disabled by configuration: no endpoint set")
		return nil, nil
	}
	return pipelines.NewMetricsPipeline(pipelines.PipelineConfig{
		Endpoint:        c.MetricExporterEndpoint,
		Insecure:        c.MetricExporterEndpointInsecure,
		Headers:         c.Headers,
		Resource:        c.Resource,
		ReportingPeriod: c.MetricReportingPeriod,
	})
}

func ConfigureOpentelemetry(opts ...Option) Launcher {
	c := newConfig(opts...)

	if c.LogLevel == "debug" {
		c.logger.Debugf("debug logging enabled")
		c.logger.Debugf("configuration")
		s, _ := json.MarshalIndent(c, "", "\t")
		c.logger.Debugf(string(s))
	}

	if c.Headers == nil {
		c.Headers = map[string]string{}
	}

	err := validateConfiguration(c)
	if err != nil {
		c.logger.Fatalf("configuration error: %v", err)
	}

	if c.errorHandler != nil {
		otel.SetErrorHandler(c.errorHandler)
	}

	ls := Launcher{
		config: c,
	}

	for _, setup := range []setupFunc{setupTracing, setupMetrics} {
		shutdown, err := setup(c)
		if err != nil {
			c.logger.Fatalf("setup error: %v", err)
			continue
		}
		if shutdown != nil {
			ls.shutdownFuncs = append(ls.shutdownFuncs, shutdown)
		}
	}
	return ls
}

func (ls Launcher) Shutdown() {
	for _, shutdown := range ls.shutdownFuncs {
		if err := shutdown(); err != nil {
			ls.config.logger.Fatalf("failed to stop exporter: %v", err)
		}
	}
}
