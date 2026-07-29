// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
)

// MemConnection implementiert transport.Connection für In-Memory Tests
type MemConnection struct {
	In  chan []byte
	Out chan []byte
}

func (m *MemConnection) Send(ctx context.Context, data []byte) error {
	select {
	case m.Out <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *MemConnection) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data, ok := <-m.In:
		if !ok {
			return nil, errors.New("connection closed")
		}
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *MemConnection) Close(reason string) error {
	// In a real test, one would be cautious here,
	// to avoid "close of closed channel".
	return nil
}

// NewMemPair creates two fully connected endpoints.
func NewMemPair() (*MemConnection, *MemConnection) {
	aToB := make(chan []byte, 10)
	bToA := make(chan []byte, 10)

	return &MemConnection{In: bToA, Out: aToB},
		&MemConnection{In: aToB, Out: bToA}
}
