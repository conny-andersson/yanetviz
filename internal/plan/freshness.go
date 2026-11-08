package plan

import "time"

// ObservationMaxAge tolerates three missed 60-second polling intervals before
// retained measurements become stale. Restarted observations still need a poll.
const ObservationMaxAge = 3 * time.Minute
