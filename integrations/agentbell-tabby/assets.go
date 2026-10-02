// Package tabbyassets bundles the Tabby integration with the AgentBell binary.
package tabbyassets

import "embed"

//go:embed package.json index.js bridge.js context.js
var Files embed.FS
