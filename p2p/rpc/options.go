// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package rpc

import "time"

// Configuration constants
const (
	defaultAddr              = ":8080"
	defaultHeartbeatInterval = 15 * time.Second
	defaultShutdownDelay     = 5 * time.Second
	defaultWriteReadLimit    = 1024 * 1024
)

type Options struct {
	Addr              string
	HeartbeatInterval time.Duration
	ShutdownDelay     time.Duration
	WriteReadLimit    int64
	Delegate          NexDelegate
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
	if o.WriteReadLimit == 0 {
		o.WriteReadLimit = defaultWriteReadLimit
	}
	if o.Delegate == nil {
		o.Delegate = &DefaultNexDelegate{}
	}
}
