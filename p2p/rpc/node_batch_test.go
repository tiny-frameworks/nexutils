package rpc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

type AddParams struct {
	A int `json:"a"`
	B int `json:"b"`
}

// BatchTestDelegate verarbeitet Anfragen für den Batch-Test
type BatchTestDelegate struct {
	rpc.DefaultNexDelegate
}

func (d *BatchTestDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	switch method {
	case "ping":
		return "pong", nil

	case "add":
		var p AddParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpc.JsonRPCerror{Code: rpc.InvalidParams, Message: rpc.StdError[rpc.InvalidParams]}
		}
		return p.A + p.B, nil

	default:
		return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
	}
}

func TestNodeBatchProcessing(t *testing.T) {
	// 1. Server-Node starten
	serverOpts := rpc.Options{
		Addr:     "127.0.0.1:8998",
		Delegate: &BatchTestDelegate{},
	}
	serverNode := rpc.NewNode(serverOpts)

	go func() {
		_ = serverNode.Start()
	}()
	defer serverNode.Stop()

	time.Sleep(50 * time.Millisecond)

	// 2. Client-Node starten & verbinden
	clientNode := rpc.NewNode(rpc.Options{})
	defer clientNode.Stop()

	peer, err := clientNode.ConnectToPeer("ws://127.0.0.1:8998/ws")
	if err != nil {
		t.Fatalf("Failed to connect to peer: %v", err)
	}
	defer peer.Close()

	// 3. Batch-Items vorbereiten
	var pingRes string
	var addRes int
	var unknownRes string

	batchItems := []rpc.BatchItem{
		{
			Method: "ping",
			Params: nil,
			Result: &pingRes,
		},
		{
			Method: "add",
			Params: AddParams{A: 12, B: 18},
			Result: &addRes,
		},
		{
			Method: "unknownMethod",
			Params: nil,
			Result: &unknownRes,
		},
	}

	// 4. CallBatch ausführen
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rpcErrors := peer.CallBatch(ctx, batchItems)

	if len(rpcErrors) != len(batchItems) {
		t.Fatalf("Expected %d error slots in result, got %d", len(batchItems), len(rpcErrors))
	}

	// 5. Ergebnisse überprüfen

	// Item 0: ping -> pong (Kein Fehler)
	if rpcErrors[0] != nil {
		t.Errorf("Unexpected error for Item 0 (ping): %v", rpcErrors[0].Message)
	}
	if pingRes != "pong" {
		t.Errorf("Item 0 result expected 'pong', got '%s'", pingRes)
	}

	// Item 1: add -> 30 (Kein Fehler)
	if rpcErrors[1] != nil {
		t.Errorf("Unexpected error for Item 1 (add): %v", rpcErrors[1].Message)
	}
	if addRes != 30 {
		t.Errorf("Item 1 result expected 30, got %d", addRes)
	}

	// Item 2: unknownMethod -> Sollte MethodNotFound Error liefern
	if rpcErrors[2] == nil {
		t.Errorf("Expected error for Item 2 (unknownMethod), but got nil")
	} else if rpcErrors[2].Code != rpc.MethodNotFound {
		t.Errorf("Expected MethodNotFound (-32601), got code %d (%s)", rpcErrors[2].Code, rpcErrors[2].Message)
	}
}
