package rpc

import "time"

// Configuration constants
const (
	defaultAddr              = ":8080"
	defaultHeartbeatInterval = 15 * time.Second
	defaultShutdownDelay     = 5 * time.Second
)

type Options struct {
	Addr              string
	HeartbeatInterval time.Duration
	ShutdownDelay     time.Duration
}

func (o *Options) setDefaults() {
	if o.Addr == "" {
		o.Addr = defaultAddr
	}
	if o.HeartbeatInterval == 0 {
		o.HeartbeatInterval = defaultHeartbeatInterval
	}
	if o.ShutdownDelay == 0 {
		o.ShutdownDelay = defaultShutdownDelay
	}
}
