package rpc_test

import (
	"context"
	"testing"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

type ErrorTestDelegate struct {
	rpc.DefaultNexDelegate
}

func (d *ErrorTestDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
	if method == "triggerNexError" {
		return nil, errors.New(
			errors.InternalError,
			"custom domain failure",
			"p2p.rpc.rpc_test")
	}
	return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
}

func TestNode_NexErrorHandling(t *testing.T) {
	// Testet die direkte Umwandlung von nexerrors.Error in JsonRPCerror
	nexErr := errors.New(
		errors.InternalError,
		"custom domain failure",
		"p2p.rpc.rpc_test")
	rpcErr := rpc.NewRPCErrorFromNexError(rpc.InternalError, nexErr)

	if rpcErr.Code != rpc.InternalError {
		t.Errorf("Erwarteter Error-Code %d, erhalten %d", rpc.InternalError, rpcErr.Code)
	}

	if rpcErr.Data == nil {
		t.Error("Erwartet: Data-Feld im JsonRPCerror befüllt mit nexerrors.Error")
	}
}
