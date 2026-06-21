// Package kernel contains Genesis' transport-neutral agent kernel.
//
// It must stay independent of CLI, WebUI, desktop UI, external channel daemons,
// and application-specific integrations. Shells and applications call the
// kernel through stable contracts; they do not define kernel behavior.
package kernel
