package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// helper structure for YAML decoding of StringMap
type cfgWithMap struct {
	M StringMap `yaml:"m"`
}

func TestStringMapUnmarshalYAML_VariousFormats(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		var c cfgWithMap
		err := yaml.Unmarshal([]byte("m: ''\n"), &c)
		require.NoError(t, err)
		require.NotNil(t, c.M)
		require.Len(t, c.M, 0)
	})

	t.Run("json string", func(t *testing.T) {
		var c cfgWithMap
		err := yaml.Unmarshal([]byte("m: '{\"k1\":\"v1\",\"k2\":\"v2\"}'\n"), &c)
		require.NoError(t, err)
		require.Equal(t, StringMap{"k1": "v1", "k2": "v2"}, c.M)
	})

	t.Run("csv string", func(t *testing.T) {
		var c cfgWithMap
		err := yaml.Unmarshal([]byte("m: 'a=1, b=2, c = 3'\n"), &c)
		require.NoError(t, err)
		require.Equal(t, StringMap{"a": "1", "b": "2", "c": "3"}, c.M)
	})

	t.Run("yaml map", func(t *testing.T) {
		var c cfgWithMap
		err := yaml.Unmarshal([]byte("m:\n  x: 10\n  y: true\n  z: val\n"), &c)
		require.NoError(t, err)
		// values are stringified by UnmarshalYAML
		require.Equal(t, StringMap{"x": "10", "y": "true", "z": "val"}, c.M)
	})
}

func TestInitTracing_HTTP_NoopUsage(t *testing.T) {
	// Use HTTP protocol to avoid opening a gRPC connection in tests.
	cfg := &Config{
		Enabled:     true,
		ServiceName: "unla-test",
		Protocol:    "http",
		// leave Endpoint empty to exercise default path (http://localhost:4318)
		Insecure:    true,
		SamplerRate: 2.5, // will be clamped to 1.0
		Environment: "dev",
		Headers:     map[string]string{"x-test": "1"},
	}

	shutdown, err := InitTracing(context.Background(), cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Do not create spans here to avoid any export attempts.
	// Ensure global propagator/provider are set and then shutdown cleanly.
	_ = otel.GetTextMapPropagator()

	// Shutdown should not error when no spans were exported.
	require.NoError(t, shutdown(context.Background()))
}

func TestBuilder_Start_WithAttrs_End_WithInMemoryProvider(t *testing.T) {
	// Set up an in-memory recorder to validate span creation and attributes.
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sr),
		sdktrace.WithResource(resource.Empty()),
	)
	// Save the previous provider and restore after test to avoid global leakage.
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	b := Tracer("trace-test")
	scope := b.Start(context.Background(), "op")
	require.NotNil(t, scope)
	scope = scope.WithAttrs(attribute.String("k", "v"))
	require.NotNil(t, scope)
	scope.End()

	// Validate one span recorded with the attribute.
	spans := sr.Ended()
	require.Len(t, spans, 1)

	attrs := spans[0].Attributes()
	// Find our attribute
	found := false
	for _, a := range attrs {
		if a.Key == "k" && a.Value.AsString() == "v" {
			found = true
			break
		}
	}
	require.True(t, found, "expected attribute k=v to be set on span")
}
