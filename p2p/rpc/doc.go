// Copyright 2026 Georg Hagn
// SPDX-License-Identifier: Apache-2.0

/*
The rpc package implements a bidirectional JSON-RPC 2.0 protocol using abstract connections.

Unlike traditional client-server RPC, this package allows symmetrical
peer-to-peer communication. Each endpoint can both register methods (server role)
and call methods on the partner (client role).

Key features:
- Support for call (request/response) and notify (fire-and-forget).
- Robust error handling with standardized JSON-RPC error codes.
- Generic bind function for type-safe unmarshaling without reflection.
- Support for automatic reconnect logic on connection failure.

Example of registering a handler:

	node.Register("sum", func(ctx context.Context, params json.RawMessage) (any, error) {
	    vals, _ := rpc.Bind[[]int](params)
	    return vals[0] + vals[1], nil
	})
*/
package rpc
