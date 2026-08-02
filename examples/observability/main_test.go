package main

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"disabled", nil, false}, {"logs", []string{"-observe", "-logs"}, false}, {"all", []string{"-observe", "-all"}, false},
		{"missing master", []string{"-logs"}, true}, {"probe without health", []string{"-observe", "-probe"}, true}, {"bad scenario", []string{"-scenario=bad"}, true}, {"bad timeout", []string{"-timeout=0s"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestRunNormal(t *testing.T) {
	for _, args := range [][]string{nil, {"-observe", "-diagnostics"}, {"-observe", "-all"}} {
		if err := run(args); err != nil {
			t.Fatalf("run(%v): %v", args, err)
		}
	}
}

func TestRunFailureScenarios(t *testing.T) {
	for _, scenario := range []string{"connect-failure", "publish-failure"} {
		if err := run([]string{"-observe", "-all", "-scenario=" + scenario}); err == nil {
			t.Fatalf("%s did not fail", scenario)
		}
	}
	if err := run([]string{"-observe", "-all", "-scenario=handler-failure"}); err != nil {
		t.Fatalf("handler scenario: %v", err)
	}
}

func TestScenarioOutputMatrixAndCleanup(t *testing.T) {
	before := runtime.NumGoroutine()
	tests := []struct {
		scenario string
		wantErr  bool
		want     []string
	}{
		{"normal", false, []string{"health state=unknown", "metric name=broker.operations", "snapshot broker=noop", "correlation=enabled", "event kind="}},
		{"connect-failure", true, []string{"snapshot broker=noop", "error_category=connection"}},
		{"publish-failure", true, []string{"snapshot broker=noop", "error_category=publish"}},
		{"handler-failure", false, []string{"snapshot broker=noop", "error_category=handler"}},
	}
	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			var output bytes.Buffer
			err := runTo([]string{"-observe", "-all", "-scenario=" + tc.scenario}, &output)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v output=%s", err, output.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("missing %q in %q", want, output.String())
				}
			}
			if strings.Contains(output.String(), "not printed") || strings.Contains(output.String(), "secret-token") {
				t.Fatalf("sensitive output: %s", output.String())
			}
		})
	}
	time.Sleep(20 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutines before=%d after=%d", before, after)
	}
}

func TestHealthProbeSwitchOutput(t *testing.T) {
	var output bytes.Buffer
	if err := runTo([]string{"-observe", "-health", "-probe"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"health state=unknown", "health probe state=ready", "unsupported=true"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q in %q", want, output.String())
		}
	}
}
