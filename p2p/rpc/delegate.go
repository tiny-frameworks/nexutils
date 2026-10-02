package rpc

import (
	"context"
	"encoding/json"

	//	"errors"
	"sync"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/errors"
)

var ErrUnhandledMethod = errors.New(
	errors.UnhandledMethodErr,
	"unhandled method",
	"p2p.rpc.delegate",
)

type SessionData struct {
	Username string
	Expires  time.Time
}

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
type DefaultNexDelegate struct {
	Authenticator UserAuthenticator

	// In-Memory SessionStore für Reconnects über Token
	sessionMu sync.RWMutex
	sessions  map[string]SessionData
}

func (d *DefaultNexDelegate) ValidatePeer(peer *Peer) bool             { return true }
func (d *DefaultNexDelegate) OnPeerConnected(peer *Peer)               {}
func (d *DefaultNexDelegate) OnPeerDisconnected(peer *Peer, err error) {}

func (d *DefaultNexDelegate) SetAuthenticator(auth UserAuthenticator) {
	if auth != nil {
		d.Authenticator = auth
	}
}

func (d *DefaultNexDelegate) OnRequest(ctx context.Context, peer *Peer, method string, params []byte) (any, error) {
	switch method {
	case "system.auth":
		return d.handleAuth(ctx, peer, params)
		/*
			auth := d.Authenticator
			if auth == nil {
				auth = &DummyAuthenticator{}
			}

			var authParams AuthParams
			if err := json.Unmarshal(params, &authParams); err != nil {
				nexErr := errors.New(
					errors.InvalidValue,
					"invald parameter",
					"p2p.rpc.delegate_test",
				)
				return nil, NewRPCErrorFromNexError(InvalidParams, nexErr)
				//return nil, &JsonRPCerror{Code: InvalidParams, Message: StdError[InvalidParams]}
			}

			// 1. Reconnect via Token
			if authParams.Token != "" {
				d.sessionMu.RLock()
				sess, found := d.sessions[authParams.Token]
				d.sessionMu.RUnlock()

				if found && sess.Expires.After(time.Now()) {
					ttl := time.Until(sess.Expires)
					peer.SetAuth(sess.Username, authParams.Token, ttl)

					return AuthResult{
						Status:   "session restored",
						Username: sess.Username,
						Token:    authParams.Token,
					}, nil
				}

				nexErr := errors.New(
					errors.InvalidOrExpired,
					"session invalid or expired",
					"p2p.rpc.delegate",
				)
				return nil, NewRPCErrorFromNexError(InvalidRequest, nexErr)
			}

			// 2. Login via Credentials
			if authParams.Username != "" && authParams.Password != "" {
				if auth.Authenticate(ctx, authParams.Username, authParams.Password) {
					token := generateToken()
					ttl := 24 * time.Hour
					expires := time.Now().Add(ttl)

					d.sessionMu.Lock()
					if d.sessions == nil {
						d.sessions = make(map[string]SessionData)
					}
					d.sessions[token] = SessionData{
						Username: authParams.Username,
						Expires:  expires,
					}
					d.sessionMu.Unlock()

					peer.SetAuth(authParams.Username, token, ttl)

					return AuthResult{
						Status:   "authenticated",
						Token:    token,
						Username: authParams.Username,
					}, nil
				}
			}

			nexErr := errors.New(
				errors.UnauthorizedError,
				"unautorized error",
				"p2p.rpc.delegate",
			)
			return nil, NewRPCErrorFromNexError(InvalidRequest, nexErr)
		*/
	case "system.ping":
		// Liefert "pong" und optional einen serverseitigen UTC-Zeitstempel für Sync/Latenz
		return map[string]any{
			"reply": "pong",
			"time":  time.Now().UTC(),
		}, nil

	default:
		nexErr := errors.New(
			errors.UnhandledMethodErr,
			"method not found error",
			"p2p.rpc.delegate",
		)
		return nil, NewRPCErrorFromNexError(MethodNotFound, nexErr)

	}
}

func (d *DefaultNexDelegate) OnNotification(ctx context.Context, peer *Peer, method string, params []byte) {
}

func (d *DefaultNexDelegate) OnError(peer *Peer, err error) {}

func (d *DefaultNexDelegate) handleAuth(ctx context.Context, peer *Peer, params []byte) (any, error) {
	auth := d.Authenticator
	if auth == nil {
		auth = &DummyAuthenticator{}
	}

	var authParams AuthParams
	if err := json.Unmarshal(params, &authParams); err != nil {
		nexErr := errors.New(
			errors.InvalidValue,
			"invald parameter",
			"p2p.rpc.delegate_test",
		)
		return nil, NewRPCErrorFromNexError(InvalidParams, nexErr)
	}

	// 1. Reconnect via Token
	if authParams.Token != "" {
		d.sessionMu.RLock()
		sess, found := d.sessions[authParams.Token]
		d.sessionMu.RUnlock()

		if found && sess.Expires.After(time.Now()) {
			ttl := time.Until(sess.Expires)
			peer.SetAuth(sess.Username, authParams.Token, ttl)

			return AuthResult{
				Status:   "session restored",
				Username: sess.Username,
				Token:    authParams.Token,
			}, nil
		}

		nexErr := errors.New(
			errors.InvalidOrExpired,
			"session invalid or expired",
			"p2p.rpc.delegate",
		)
		return nil, NewRPCErrorFromNexError(InvalidRequest, nexErr)
	}

	// 2. Login via Credentials
	if authParams.Username != "" && authParams.Password != "" {
		if auth.Authenticate(ctx, authParams.Username, authParams.Password) {
			token := generateToken()
			ttl := 24 * time.Hour
			expires := time.Now().Add(ttl)

			d.sessionMu.Lock()
			if d.sessions == nil {
				d.sessions = make(map[string]SessionData)
			}
			d.sessions[token] = SessionData{
				Username: authParams.Username,
				Expires:  expires,
			}
			d.sessionMu.Unlock()

			peer.SetAuth(authParams.Username, token, ttl)

			return AuthResult{
				Status:   "authenticated",
				Token:    token,
				Username: authParams.Username,
			}, nil
		}
	}

	nexErr := errors.New(
		errors.UnauthorizedError,
		"unautorized error",
		"p2p.rpc.delegate",
	)
	return nil, NewRPCErrorFromNexError(InvalidRequest, nexErr)
}
