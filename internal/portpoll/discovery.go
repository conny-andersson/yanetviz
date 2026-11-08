package portpoll

import (
	"context"
	"fmt"
	"time"
	"yanetviz/internal/plan"
)

// Discover reads model and RouterOS properties at most every five minutes.
// It does not infer PoE-out capability from a marketing model or API absence.
// Discovery failure never prevents independent Ethernet/PoE polling.
func (c *Collector) Discover(ctx context.Context, now time.Time) {
	if c.metadata != nil {
		previous, _ := time.Parse(time.RFC3339Nano, c.metadata.LastAttempt)
		if now.Sub(previous) < 5*time.Minute {
			return
		}
	}
	metadata := &plan.DeviceInventory{LastAttempt: now.UTC().Format(time.RFC3339Nano)}
	resource, err := c.client.ReadResource(ctx, "system/resource")
	if err != nil {
		metadata.Error = err.Error()
		c.metadata = metadata
		return
	}
	if resource["version"] == "" {
		metadata.Error = "device resource response omitted RouterOS version"
		c.metadata = metadata
		return
	}
	metadata.OSVersion = resource["version"]
	metadata.BoardName = resource["board-name"]
	metadata.Architecture = resource["architecture-name"]
	board, err := c.client.ReadResource(ctx, "system/routerboard")
	if err != nil {
		metadata.Error = fmt.Sprintf("RouterOS properties read; hardware model unavailable: %v", err)
	} else {
		metadata.Model = board["model"]
	}
	metadata.LastSuccess = metadata.LastAttempt
	if err := metadata.Validate(); err != nil {
		metadata = &plan.DeviceInventory{LastAttempt: metadata.LastAttempt, Error: "invalid device inventory properties"}
	}
	c.metadata = metadata
}
