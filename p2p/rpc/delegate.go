package rpc

import (
	"context"
	"errors"
)

var ErrUnhandledMethod = errors.New("unhandled method")

// P2PDelegate definiert das Verhalten und die Hooks für einen P2P-Node.
type NexDelegate interface {
	// --- Lifecycle Hooks ---
	// ValidatePeer wird beim eingehenden Handshake aufgerufen.
	// Gibt false zurück, um die Verbindung sofort abzulehnen (z.B. Auth-Fehler).
	ValidatePeer(peer *Peer) bool

	// OnPeerConnected wird nach erfolgreichem Handshake aufgerufen.
	OnPeerConnected(peer *Peer)

	// OnPeerDisconnected wird beim Trennen der Verbindung aufgerufen.
	OnPeerDisconnected(peer *Peer, err error)

	// --- Message / RPC Handling ---
	// OnRequest verarbeitet eingehende RPC-Anfragen.
	OnRequest(ctx context.Context, peer *Peer, method string, params []byte) (any, error)

	// OnNotification verarbeitet eingehende Einweg-Nachrichten (ohne Response).
	OnNotification(ctx context.Context, peer *Peer, method string, params []byte)

	// --- Error Handling ---
	// OnError fängt Protokoll- oder Transportfehler ab.
	OnError(peer *Peer, err error)
}

// DefaultNexDelegate bietet eine Basisimplementierung.
// Benutzerdefinierte Delegates können dieses Struct einbetten (Embedding)
// und müssen nur die benötigten Methoden überschreiben.
type DefaultNexDelegate struct{}

func (d *DefaultNexDelegate) ValidatePeer(peer *Peer) bool             { return true }
func (d *DefaultNexDelegate) OnPeerConnected(peer *Peer)               {}
func (d *DefaultNexDelegate) OnPeerDisconnected(peer *Peer, err error) {}

func (d *DefaultNexDelegate) OnRequest(ctx context.Context, peer *Peer, method string, params []byte) (any, error) {
	return nil, &JsonRPCerror{
		Code:    MethodNotFound,
		Message: StdError[MethodNotFound],
	}
}

func (d *DefaultNexDelegate) OnNotification(ctx context.Context, peer *Peer, method string, params []byte) {
}

func (d *DefaultNexDelegate) OnError(peer *Peer, err error) {}
