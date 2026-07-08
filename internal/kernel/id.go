package kernel

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

func newID(prefix string, now time.Time) string {
	id, err := ulid.New(ulid.Timestamp(now), rand.Reader)
	if err != nil {
		return fmt.Sprintf("%s_%d", prefix, now.UnixNano())
	}
	return fmt.Sprintf("%s_%s", prefix, id.String())
}
