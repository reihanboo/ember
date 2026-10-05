package ui

import (
	"bytes"
	"testing"
	"time"
)

func TestLoggerOutput(t *testing.T) {
	fixedTime := time.Date(2025, time.April, 5, 6, 7, 8, 0, time.UTC)
	tests := []struct {
		name  string
		color bool
		log   func(*Logger)
		want  string
	}{
		{
			name: "debug without color",
			log:  func(logger *Logger) { logger.Debug("details") },
			want: "2025-04-05T06:07:08Z [DEBUG] details\n",
		},
		{
			name:  "info with color",
			color: true,
			log:   func(logger *Logger) { logger.Info("ready") },
			want:  "2025-04-05T06:07:08Z \x1b[32m[INFO]\x1b[0m ready\n",
		},
		{
			name: "warn without color",
			log:  func(logger *Logger) { logger.Warn("slow") },
			want: "2025-04-05T06:07:08Z [WARN] slow\n",
		},
		{
			name:  "error with color",
			color: true,
			log:   func(logger *Logger) { logger.Error("failed") },
			want:  "2025-04-05T06:07:08Z \x1b[31m[ERROR]\x1b[0m failed\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := NewLogger(&output, func() time.Time { return fixedTime }, test.color, DebugLevel)
			test.log(logger)
			if got := output.String(); got != test.want {
				t.Errorf("output = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	fixedTime := time.Date(2025, time.April, 5, 6, 7, 8, 0, time.UTC)
	var output bytes.Buffer
	logger := NewLogger(&output, func() time.Time { return fixedTime }, false, WarnLevel)

	logger.Debug("details")
	logger.Info("ready")
	logger.Warn("slow")
	logger.Error("failed")

	want := "2025-04-05T06:07:08Z [WARN] slow\n" +
		"2025-04-05T06:07:08Z [ERROR] failed\n"
	if got := output.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
