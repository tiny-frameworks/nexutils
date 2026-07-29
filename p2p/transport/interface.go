// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package transport

import "context"

type Connection interface {
	Send(ctx context.Context, data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	Close(reason string) error
}

type WSService interface {
	Listen(addr string, found chan<- Connection) error
	Dial(ctx context.Context, url string) (Connection, error)
}
