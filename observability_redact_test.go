package broker

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCustomRedactedFieldsExtendDefaults(t *testing.T) {
	safe := safeErrorWithFields(OperationPublish, errors.New("password=default custom-secret=hidden visible=ok"), []string{"custom-secret"})
	if strings.Contains(safe.Summary, "default") || strings.Contains(safe.Summary, "hidden") {
		t.Fatalf("secret leaked: %s", safe.Summary)
	}
	if !strings.Contains(safe.Summary, "visible=ok") {
		t.Fatalf("safe context removed: %s", safe.Summary)
	}
}

func TestObservabilityOutputsExcludeSensitiveFixtures(t *testing.T) {
	fixtures := []string{
		"amqp://admin:password@broker/vhost",
		"https://service/path?X-Amz-Signature=abcdef&token=secret-token",
		"api_key=private password=hunter2",
		"-----BEGIN PRIVATE KEY-----key-material-----END PRIVATE KEY-----",
	}
	for _, fixture := range fixtures {
		summary := sanitizeSummary("vendor failure " + fixture)
		for _, secret := range []string{"admin:password", "abcdef", "secret-token", "private", "hunter2", "key-material"} {
			if strings.Contains(summary, secret) {
				t.Fatalf("fixture %q leaked %q in %q", fixture, secret, summary)
			}
		}
	}
	runtime, _ := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryDiagnostics), BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: 1, ProbeTimeout: 1, RedactedFields: []string{"tenant-secret"}, CorrelationConflict: CorrelationPreserve})
	runtime.Observe(OperationPublish, time.Now(), errors.New("tenant-secret=hidden"))
	snapshot := runtime.Snapshot()
	if strings.Contains(snapshot.LastFailure.Summary, "hidden") {
		t.Fatal("custom header value leaked")
	}
	if strings.Contains(fmt.Sprint(snapshot), "message-body-fixture") {
		t.Fatal("message body appeared in snapshot")
	}
}

func FuzzObservabilityRedaction(f *testing.F) {
	for _, seed := range []string{"password=hunter2", "token=abc", "https://user:pass@example.test", "-----BEGIN PRIVATE KEY-----secret-----END PRIVATE KEY-----"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		result := sanitizeSummary(input)
		if len(result) > 515 {
			t.Fatalf("summary not bounded: %d", len(result))
		}
	})
}
