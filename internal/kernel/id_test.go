package kernel

import (
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func TestNewIDUsesPrefixedULIDWithInjectedTime(t *testing.T) {
	firstTime := time.Date(2026, 7, 8, 1, 2, 3, 0, time.UTC)
	secondTime := firstTime.Add(time.Millisecond)

	first := newID("evt", firstTime)
	second := newID("evt", secondTime)

	if !strings.HasPrefix(first, "evt_") || !strings.HasPrefix(second, "evt_") {
		t.Fatalf("ids = %q, %q, want evt_ prefix", first, second)
	}
	firstULID, err := ulid.ParseStrict(strings.TrimPrefix(first, "evt_"))
	if err != nil {
		t.Fatalf("first id is not a strict ULID: %q: %v", first, err)
	}
	secondULID, err := ulid.ParseStrict(strings.TrimPrefix(second, "evt_"))
	if err != nil {
		t.Fatalf("second id is not a strict ULID: %q: %v", second, err)
	}
	if !ulid.Time(firstULID.Time()).Equal(firstTime) || !ulid.Time(secondULID.Time()).Equal(secondTime) {
		t.Fatalf("ulid times = %s, %s; want %s, %s", ulid.Time(firstULID.Time()), ulid.Time(secondULID.Time()), firstTime, secondTime)
	}
	if strings.Compare(first, second) >= 0 {
		t.Fatalf("ids are not time-sortable: %q >= %q", first, second)
	}
}
