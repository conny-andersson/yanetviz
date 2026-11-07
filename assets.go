package yanetviz

import "embed"

// Public assets only. Never embed inventory.local.yaml or local application data.
//
//go:embed web/dist floorplans
var Assets embed.FS
