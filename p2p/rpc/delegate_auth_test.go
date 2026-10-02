package rpc_test

import (
	"context"
	"encoding/json"
	"testing"

	"codeberg.org/tiny-frameworks/nexutils/errors"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

// CustomAuth implementiert rpc.UserAuthenticator für benutzerdefinierte Tests
type CustomAuth struct {
	users map[string]string
}

func (a *CustomAuth) Authenticate(ctx context.Context, username, password string) bool {
	pwd, ok := a.users[username]
	return ok && pwd == password
}

func TestDefaultNexDelegate_Auth(t *testing.T) {
	ctx := context.Background()

	t.Run("Session Restoring über Token", func(t *testing.T) {
		// DIESELBE Delegate-Instanz für beide Schritte!
		delegate := &rpc.DefaultNexDelegate{}

		// 1. Login via Credentials auf peer1
		peer1 := rpc.NewPeer(nil, "127.0.0.1:1234")
		loginParams, _ := json.Marshal(rpc.AuthParams{
			Username: "georg",
			Password: "secret",
		})

		res1, err1 := delegate.OnRequest(ctx, peer1, "system.auth", loginParams)
		if err1 != nil {
			t.Fatalf("Erst-Login fehlgeschlagen: %v", err1)
		}

		authRes1, ok := res1.(rpc.AuthResult)
		if !ok || authRes1.Token == "" {
			t.Fatalf("Ungültiges Login-Ergebnis: %+v", res1)
		}

		// 2. Reconnect via Token auf NEUEM Peer (peer2) über DIESELBE Delegate-Instanz
		peer2 := rpc.NewPeer(nil, "127.0.0.1:1234")
		tokenParams, _ := json.Marshal(rpc.AuthParams{
			Token: authRes1.Token,
		})

		res2, err2 := delegate.OnRequest(ctx, peer2, "system.auth", tokenParams)
		if err2 != nil {
			t.Fatalf("Fehler beim Token-Reconnect: %v", err2)
		}

		authRes2 := res2.(rpc.AuthResult)
		if authRes2.Status != "session restored" {
			t.Errorf("Erwarteter Status 'session restored', erhalten: %s", authRes2.Status)
		}

		if authRes2.Username != "georg" {
			t.Errorf("Erwarteter Username 'georg', erhalten: %s", authRes2.Username)
		}

		if !peer2.IsAuthorized() {
			t.Error("peer2 sollte nach Token-Restore autorisiert sein")
		}
	})

	t.Run("Standard DummyAuthenticator - Ungültige Anmeldedaten", func(t *testing.T) {
		delegate := &rpc.DefaultNexDelegate{}
		peer := rpc.NewPeer(nil, "127.0.0.1:1234")

		params, _ := json.Marshal(rpc.AuthParams{
			Username: "georg",
			Password: "falsches-passwort",
		})

		_, err := delegate.OnRequest(ctx, peer, "system.auth", params)
		if err == nil {
			t.Fatal("Erwartet: UnAuthorized-Fehler, erhalten: nil")
		}

		rpcErr, ok := err.(*rpc.JsonRPCerror)
		if !ok {
			t.Fatalf("Erwartet *rpc.JsonRPCerror, erhalten: %T (%v)", err, err)
		}
		// Entpacken des nexerrors.Error aus rpcErr.Data
		nexErr, ok := rpcErr.Data.(*errors.Error)
		if !ok {
			t.Fatalf("rpcErr.Data enthält keinen *errors.Error, erhalten: %T", rpcErr.Data)
		}
		if nexErr.Code != errors.UnauthorizedError {
			t.Errorf("Erwarteter nexerror.Code %v, erhalten: %v", errors.UnauthorizedError, nexErr.Code)
		}

		if peer.IsAuthorized() {
			t.Error("Peer darf bei fehlgeschlagenem Login nicht autorisiert sein")
		}
	})

	t.Run("Custom Authenticator Injektion", func(t *testing.T) {
		customAuth := &CustomAuth{
			users: map[string]string{
				"admin": "nexfact-secret",
			},
		}

		delegate := &rpc.DefaultNexDelegate{}
		delegate.SetAuthenticator(customAuth)

		peer := rpc.NewPeer(nil, "127.0.0.1:1234")

		params, _ := json.Marshal(rpc.AuthParams{
			Username: "admin",
			Password: "nexfact-secret",
		})

		res, err := delegate.OnRequest(ctx, peer, "system.auth", params)
		if err != nil {
			t.Fatalf("Fehler bei Custom Authenticator Login: %v", err)
		}

		authRes := res.(rpc.AuthResult)
		if authRes.Username != "admin" {
			t.Errorf("Erwarteter Username 'admin', erhalten: %s", authRes.Username)
		}
	})

	t.Run("Ungültiges Token Reconnect", func(t *testing.T) {
		delegate := &rpc.DefaultNexDelegate{}
		peer := rpc.NewPeer(nil, "127.0.0.1:1234") // Peer ist NICHT autorisiert

		params, _ := json.Marshal(rpc.AuthParams{
			Token: "invalid-token",
		})

		_, err := delegate.OnRequest(ctx, peer, "system.auth", params)
		if err == nil {
			t.Fatal("Erwartet: InvalidOrExpired-Fehler, erhalten: nil")
		}

		rpcErr, ok := err.(*rpc.JsonRPCerror)
		if !ok {
			t.Fatalf("Erwartet *rpc.JsonRPCerror, erhalten: %T (%v)", err, err)
		}
		// Entpacken des nexerrors.Error aus rpcErr.Data
		nexErr, ok := rpcErr.Data.(*errors.Error)
		if !ok {
			t.Fatalf("rpcErr.Data enthält keinen *errors.Error, erhalten: %T", rpcErr.Data)
		}
		if nexErr.Code != errors.InvalidOrExpired {
			t.Errorf("Erwarteter nexerror.Code %v, erhalten: %v", errors.InvalidOrExpired, nexErr.Code)
		}

	})

	t.Run("Unbekannte RPC Methode - Fallback zu MethodNotFound", func(t *testing.T) {
		delegate := &rpc.DefaultNexDelegate{}
		peer := rpc.NewPeer(nil, "127.0.0.1:1234")

		_, err := delegate.OnRequest(ctx, peer, "unknown.method", nil)
		if err == nil {
			t.Fatal("Erwartet: MethodNotFound-Fehler, erhalten: nil")
		}

		rpcErr, ok := err.(*rpc.JsonRPCerror)
		if !ok {
			t.Fatalf("Erwartet *rpc.JsonRPCerror, erhalten: %T (%v)", err, err)
		}
		// Entpacken des nexerrors.Error aus rpcErr.Data
		nexErr, ok := rpcErr.Data.(*errors.Error)
		if !ok {
			t.Fatalf("rpcErr.Data enthält keinen *errors.Error, erhalten: %T", rpcErr.Data)
		}
		if nexErr.Code != errors.UnhandledMethodErr {
			t.Errorf("Erwarteter nexerror.Code %v, erhalten: %v", errors.UnhandledMethodErr, nexErr.Code)
		}
	})
}
