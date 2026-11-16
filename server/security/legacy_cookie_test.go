package security

import (
	http "net/http"
	httptest "net/http/httptest"
	filepath "path/filepath"
	strings "strings"
	testing "testing"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	config "rook/server/config"
	db "rook/server/db"
	encryption "rook/server/encryption"
	models "rook/server/models"
)

// legacyFixture is a guard over a fresh database with one signed-in user, and
// an app that answers who the request is.
type legacyFixture struct {
	guard      *Guard
	app        *fiber.App
	credential string
}

func newLegacyFixture( t *testing.T ) ( fixture *legacyFixture ) {
	t.Helper()
	cfg := &config.Config{
		SecretKey:     encryption.GenerateKeyHex(),
		DatabasePath:  filepath.Join( t.TempDir() , "app.db" ),
		EncryptAtRest: true,
		CookieName:    "session",
		SecureCookies: true,
		SessionTTL:    time.Hour,
	}
	store , err := db.Open( cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( func() { store.Close() } )
	user , err := models.CreateUser( store , "Alice" , models.RoleUser )
	if err != nil { t.Fatal( err ) }
	credential , _ , err := models.CreateSession( store , user.ID , time.Hour )
	if err != nil { t.Fatal( err ) }

	guard := New( store , cfg )
	app := fiber.New()
	app.Use( guard.LoadUser )
	app.Get( "/whoami" , func( c fiber.Ctx ) ( err error ) {
		name := "nobody"
		if actor := ActorFrom( c ); actor != nil { name = actor.User.DisplayName }
		err = c.SendString( name )
		return
	} )
	fixture = &legacyFixture{ guard: guard , app: app , credential: credential }
	return
}

func ( fixture *legacyFixture ) get( t *testing.T , cookie *http.Cookie ) ( who string , response *http.Response ) {
	t.Helper()
	request := httptest.NewRequest( http.MethodGet , "/whoami" , nil )
	if cookie != nil { request.AddCookie( cookie ) }
	response , err := fixture.app.Test( request )
	if err != nil { t.Fatal( err ) }
	body := make( []byte , 64 )
	count , _ := response.Body.Read( body )
	who = string( body[ :count ] )
	return
}

func cookieNamed( response *http.Response , name string ) ( found *http.Cookie ) {
	for _ , cookie := range response.Cookies() {
		if cookie.Name == name { found = cookie }
	}
	return
}

// A browser signed in before the cookie moved to __Host-session and the
// cookie subkey (go-webserver S12, S16) stays signed in: its old cookie is
// accepted once and swapped for the new one on the same response.
func TestALegacySessionCookieIsUpgradedInPlace( t *testing.T ) {
	fixture := newLegacyFixture( t )
	cfg := fixture.guard.cfg()
	if cfg.SessionCookieName() != "__Host-session" {
		t.Fatalf( "the cookie should be host-prefixed while secure, got %q" , cfg.SessionCookieName() )
	}

	// The cookie as the old code wrote it: sealed with secret.key itself.
	old_sealed , err := encryption.ChaChaEncryptString( cfg.SecretKey , fixture.credential )
	if err != nil { t.Fatal( err ) }
	who , response := fixture.get( t , &http.Cookie{ Name: "session" , Value: old_sealed } )
	if who != "Alice" {
		t.Fatalf( "the old cookie should still sign in, got %q" , who )
	}
	upgraded := cookieNamed( response , "__Host-session" )
	if upgraded == nil || upgraded.Value == "" {
		t.Fatal( "the response should set the new __Host-session cookie" )
	}
	if upgraded.Secure == false || upgraded.Path != "/" || upgraded.HttpOnly == false {
		t.Fatalf( "the new cookie needs Secure, Path=/ and HttpOnly: %+v" , upgraded )
	}
	expired := cookieNamed( response , "session" )
	if expired == nil || expired.Value != "" || ( expired.MaxAge >= 0 && expired.Expires.After( time.Now() ) ) {
		t.Fatalf( "the response should expire the old cookie, got %+v" , expired )
	}

	// The new cookie on its own is enough from now on, and the old one is no
	// longer needed.
	who , response = fixture.get( t , &http.Cookie{ Name: "__Host-session" , Value: upgraded.Value } )
	if who != "Alice" {
		t.Fatalf( "the new cookie should sign in, got %q" , who )
	}
	if strings.Contains( response.Header.Get( "Set-Cookie" ) , "session=" ) {
		t.Fatalf( "a current cookie needs no rewriting, got %q" , response.Header.Get( "Set-Cookie" ) )
	}
}

// The fallback is only for the old cookie, not a way around the subkey: a
// new-name cookie sealed the old way, or a forged one, signs nobody in.
func TestOnlyARealOldCookieIsAccepted( t *testing.T ) {
	fixture := newLegacyFixture( t )
	cfg := fixture.guard.cfg()
	old_sealed , _ := encryption.ChaChaEncryptString( cfg.SecretKey , fixture.credential )
	if who , _ := fixture.get( t , &http.Cookie{ Name: "__Host-session" , Value: old_sealed } ); who != "nobody" {
		t.Fatalf( "a __Host- cookie must be sealed with the cookie subkey, got %q" , who )
	}
	other_key , _ := encryption.ChaChaEncryptString( encryption.GenerateKeyHex() , fixture.credential )
	if who , _ := fixture.get( t , &http.Cookie{ Name: "session" , Value: other_key } ); who != "nobody" {
		t.Fatalf( "an old cookie sealed with another key must not sign in, got %q" , who )
	}
	if who , _ := fixture.get( t , nil ); who != "nobody" {
		t.Fatalf( "no cookie is nobody, got %q" , who )
	}
}
