// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"

	"github.com/coder/websocket"
)

type WSConnection struct {
	Conn *websocket.Conn
}

func (w *WSConnection) Send(ctx context.Context, data []byte) error {
	return w.Conn.Write(ctx, websocket.MessageText, data)
}

func (w *WSConnection) Receive(ctx context.Context) ([]byte, error) {
	_, data, err := w.Conn.Read(ctx)
	return data, err
}

func (w *WSConnection) Close(reason string) error {
	return w.Conn.Close(websocket.StatusNormalClosure, reason)
}
