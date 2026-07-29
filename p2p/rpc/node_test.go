package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/p2p/transport"
)

func TestPeerRPC(t *testing.T) {
	// Create a context that we can explicitly cancel.
	ctx, cancel := context.WithCancel(context.Background())

	// t.Cleanup ensures that the goroutine is stopped at the end of the test.
	t.Cleanup(func() {
		cancel()
		time.Sleep(5 * time.Millisecond) // Small buffer for "swinging off"
	})

	// 1. Two channels for bidirectional communication
	ch1 := make(chan []byte, 10)
	ch2 := make(chan []byte, 10)

	// 2.Create two connected transport ends
	clientConn := &transport.MemConnection{In: ch1, Out: ch2}
	serverConn := &transport.MemConnection{In: ch2, Out: ch1}

	serverNode := NewNode(serverConn, nil, "", nil)

	// 3.Start server loop in background
	go serverNode.Listen(ctx)

	// 4. Test-Case: Ping
	serverNode.Register("ping", func(ctx context.Context, p json.RawMessage) (any, error) {
		return "pong", nil
	})

	t.Run("Ping-Pong", func(t *testing.T) {
		req := Request{
			JSONRPC: "2.0",
			Method:  "ping",
			ID:      json.RawMessage(`1`),
		}
		reqBytes, _ := json.Marshal(req)

		// send request
		clientConn.Send(ctx, reqBytes)

		// receive answer
		respBytes, err := clientConn.Receive(ctx)
		if err != nil {
			t.Fatalf("Failed to receive: %v", err)
		}

		var resp Response
		json.Unmarshal(respBytes, &resp)

		if string(resp.Result) != `"pong"` {
			t.Errorf("Expected pong, got %s", string(resp.Result))
		}
	})
	serverNode.Register("echo", func(ctx context.Context, p json.RawMessage) (any, error) {
		return p, nil
	})

	t.Run("Echo-Complex", func(t *testing.T) {
		complexData := `{"foo":"bar","value":42}`
		req := Request{
			JSONRPC: JRPCVERSION,
			Method:  "echo",
			Params:  json.RawMessage(complexData),
			ID:      json.RawMessage(`2`),
		}
		reqBytes, _ := json.Marshal(req)

		clientConn.Send(ctx, reqBytes)
		respBytes, _ := clientConn.Receive(ctx)

		var resp Response
		json.Unmarshal(respBytes, &resp)

		if string(resp.Result) != complexData {
			t.Errorf("Expected %s, got %s", complexData, string(resp.Result))
		}
	})

}

func TestBidirectionalCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // defer is sufficient here, as the test has no sub-tests.

	ch1 := make(chan []byte, 10)
	ch2 := make(chan []byte, 10)
	clientConn := &transport.MemConnection{In: ch1, Out: ch2}
	serverConn := &transport.MemConnection{In: ch2, Out: ch1}

	serverNode := NewNode(serverConn, nil, "", nil)
	clientNode := NewNode(clientConn, nil, "", nil)

	// IMPORTANT: The server must register in order to respond to the client's call.
	serverNode.Register("ping", func(ctx context.Context, p json.RawMessage) (any, error) {
		return "pong", nil
	})

	go serverNode.Listen(ctx)
	go clientNode.Listen(ctx)

	// The client calls the server.
	res, err := clientNode.Call(ctx, "ping", nil)

	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if string(res) != `"pong"` {
		t.Errorf("Expected pong, got %s", string(res))
	}
}
