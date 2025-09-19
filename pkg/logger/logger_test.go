package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestGetLogLevel(t *testing.T) {
	cases := map[string]zapcore.Level{
		"debug":   zapcore.DebugLevel,
		"info":    zapcore.InfoLevel,
		"warn":    zapcore.WarnLevel,
		"error":   zapcore.ErrorLevel,
		"dpanic":  zapcore.DPanicLevel,
		"panic":   zapcore.PanicLevel,
		"fatal":   zapcore.FatalLevel,
		"unknown": zapcore.InfoLevel, // default
	}
	for in, exp := range cases {
		assert.Equal(t, exp, getLogLevel(in))
	}
}

func TestSetDefaultsAndEncoderAndNewLogger(t *testing.T) {
	cfg := &config.LoggerConfig{}
	// defaults
	setLoggerDefaults(cfg)
	assert.Equal(t, "info", cfg.Level)
	assert.Equal(t, "json", cfg.Format)
	assert.Equal(t, "stdout", cfg.Output)
	assert.Equal(t, 100, cfg.MaxSize)
	assert.Equal(t, 3, cfg.MaxBackups)
	assert.Equal(t, 7, cfg.MaxAge)
	assert.NotEmpty(t, cfg.TimeZone)
	assert.NotEmpty(t, cfg.TimeFormat)

	// encoder returns non-nil
	enc := getEncoder(cfg)
	assert.NotNil(t, enc)

	// stdout logger
	lg, err := NewLogger(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, lg)

	// file logger writer
	tmp := t.TempDir()
	cfg2 := &config.LoggerConfig{Output: "file", FilePath: filepath.Join(tmp, "app.log"), Format: "console", Color: true}
	setLoggerDefaults(cfg2)
	ws := getLogWriter(cfg2)
	assert.NotNil(t, ws)
	// ensure directory exists
	_, err = os.Stat(tmp)
	assert.NoError(t, err)

	// resolve time zone uses sync.Once; simulate a stable location
	cfg2.TimeZone = "UTC"
	loc := resolveTimeZone(cfg2)
	assert.Equal(t, "UTC", loc.String())
	// time formatting path does not panic
	_ = getEncoder(cfg2)
}

func TestNewLogger_FileWithStacktrace(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.LoggerConfig{
		Output:     "file",
		FilePath:   filepath.Join(tmp, "app.log"),
		Format:     "console",
		Color:      true,
		Stacktrace: true,
		Level:      "debug",
		TimeZone:   "UTC",
	}
	setLoggerDefaults(cfg)

	lg, err := NewLogger(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, lg)

	// emit a couple of logs to exercise the core paths
	lg.Debug("debug message")
	lg.Error("error message")

	// ensure file path directory exists (created by NewLogger)
	_, err = os.Stat(filepath.Dir(cfg.FilePath))
	assert.NoError(t, err)
}
