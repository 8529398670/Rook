package routes

import (
	bytes "bytes"
	json "encoding/json"
	io "io"
	http "net/http"
	strings "strings"
	testing "testing"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	config "rook/server/config"
	db "rook/server/db"
	models "rook/server/models"
	security "rook/server/security"
	tables "rook/server/tables"
)

// keyTestApp is the API with nothing else mounted: enough to call routes with
// an API key.
func keyTestApp( t *testing.T ) ( app *fiber.App , store *db.Store , cfg *config.Config ) {
	t.Helper()
	cfg , err := config.ForTest( t.TempDir() )
	if err != nil { t.Fatal( err ) }
	cfg.APIKeysEnabled = true
	store , err = db.Open( cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( func() { store.Close() } )
	hub , err := tables.New( store , cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( hub.Close )
	handlers := New( store , cfg , security.New( store , cfg ) , nil , nil , hub )
	app = fiber.New()
	handlers.Register( app )
	return
}

// callWithKey sends body (nil for none) with key as the bearer credential,
// and answers the status and the decoded reply.
func callWithKey( t *testing.T , app *fiber.App , key string , method string , path string , body any ) ( status int , reply map[string]any ) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded , err := json.Marshal( body )
		if err != nil { t.Fatal( err ) }
		reader = bytes.NewReader( encoded )
	}
	request , err := http.NewRequest( method , path , reader )
	if err != nil { t.Fatal( err ) }
	request.Header.Set( "Authorization" , "Bearer " + key )
	if body != nil { request.Header.Set( "Content-Type" , "application/json" ) }
	response , err := app.Test( request )
	if err != nil { t.Fatal( err ) }
	defer response.Body.Close()
	raw , _ := io.ReadAll( response.Body )
	reply = map[string]any{}
	_ = json.Unmarshal( raw , &reply )
	status = response.StatusCode
	return
}

// keyFor makes an account and one of its keys, with Sign-in links on when
// may_sign_in.
func keyFor( t *testing.T , store *db.Store , name string , role string , may_sign_in bool ) ( user *models.User , credential string , key *models.APIKey ) {
	t.Helper()
	user , err := models.CreateUser( store , name , role )
	if err != nil { t.Fatal( err ) }
	credential , key , err = models.IssueAPIKey( store , user.ID , "portal" , role , time.Hour )
	if err != nil { t.Fatal( err ) }
	if may_sign_in {
		if key , err = models.SetAPIKeySignIn( store , key.ID , user.ID , true ); err != nil { t.Fatal( err ) }
	}
	return
}

// The project portal's request, exactly.
var portalLinkRequest = map[string]any{ "max_uses": 1 , "expires_in_minutes": 15 }

func TestAKeyMakesItsOwnLoginLinkOnlyWhenAllowed( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	_ , allowed , _ := keyFor( t , store , "Ana" , models.RoleUser , true )
	_ , refused , _ := keyFor( t , store , "Ben" , models.RoleUser , false )

	status , reply := callWithKey( t , app , allowed , http.MethodPost , "/api/account/login-link" , portalLinkRequest )
	if status != http.StatusOK {
		t.Fatalf( "a key with Sign-in links on: %d %v" , status , reply )
	}
	path , _ := reply[ "login_path" ].( string )
	if strings.HasPrefix( path , "/login/" ) == false || len( path ) < 10 {
		t.Fatalf( "no login_path in %v" , reply )
	}

	status , reply = callWithKey( t , app , refused , http.MethodPost , "/api/account/login-link" , portalLinkRequest )
	if status != http.StatusForbidden {
		t.Fatalf( "a key without Sign-in links: %d %v, want 403" , status , reply )
	}
}

// Whatever a key asks for, its link works once and for 15 minutes at most.
func TestAKeysLoginLinkIsCapped( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	_ , credential , _ := keyFor( t , store , "Ana" , models.RoleUser , true )
	status , reply := callWithKey( t , app , credential , http.MethodPost , "/api/account/login-link" ,
		map[string]any{ "max_uses": 5 , "expires_in_minutes": 600 } )
	if status != http.StatusOK { t.Fatalf( "%d %v" , status , reply ) }
	if seconds , _ := reply[ "expires_in_seconds" ].( float64 ); seconds != 15*60 {
		t.Fatalf( "a key's link lasts %v seconds, want 900" , reply[ "expires_in_seconds" ] )
	}
	if uses , _ := reply[ "max_uses" ].( float64 ); uses != 1 {
		t.Fatalf( "a key's link works %v times, want 1" , reply[ "max_uses" ] )
	}
}

func TestRevokingTheKeyVoidsItsLoginLink( t *testing.T ) {
	app , store , cfg := keyTestApp( t )
	_ , credential , key := keyFor( t , store , "Ana" , models.RoleUser , true )
	link := func() ( login string ) {
		status , reply := callWithKey( t , app , credential , http.MethodPost , "/api/account/login-link" , portalLinkRequest )
		if status != http.StatusOK { t.Fatalf( "%d %v" , status , reply ) }
		login = strings.TrimPrefix( reply[ "login_path" ].( string ) , "/login/" )
		return
	}
	// One link signs a browser in; the next, made after it, is left unused.
	session , _ , err := models.RedeemLoginToken( store , link() , cfg.SessionTTL )
	if err != nil { t.Fatalf( "redeeming the key's link: %v" , err ) }
	unused := link()
	if _ , user := models.LoadSession( store , session ); user == nil {
		t.Fatal( "the key's link did not sign in" )
	}

	if err := models.RevokeAPIKey( store , key.ID ); err != nil { t.Fatal( err ) }

	if _ , _ , err := models.RedeemLoginToken( store , unused , cfg.SessionTTL ); err == nil {
		t.Fatal( "a link the revoked key made still signs in" )
	}
	if _ , user := models.LoadSession( store , session ); user != nil {
		t.Fatal( "the browser the revoked key's link signed in is still signed in" )
	}
}

// Turning Sign-in links off voids the unused link but leaves the key working.
func TestSwitchingSignInOffVoidsTheUnusedLink( t *testing.T ) {
	app , store , cfg := keyTestApp( t )
	user , credential , key := keyFor( t , store , "Ana" , models.RoleUser , true )
	_ , reply := callWithKey( t , app , credential , http.MethodPost , "/api/account/login-link" , portalLinkRequest )
	unused := strings.TrimPrefix( reply[ "login_path" ].( string ) , "/login/" )
	if _ , err := models.SetAPIKeySignIn( store , key.ID , user.ID , false ); err != nil { t.Fatal( err ) }
	if _ , _ , err := models.RedeemLoginToken( store , unused , cfg.SessionTTL ); err == nil {
		t.Fatal( "a link still works after Sign-in links went off" )
	}
	if status , _ := callWithKey( t , app , credential , http.MethodPost , "/api/account/login-link" , portalLinkRequest ); status != http.StatusForbidden {
		t.Fatalf( "after switching off: %d, want 403" , status )
	}
}

// Nobody else flips a key's Sign-in links, not even the owner.
func TestOnlyTheKeysAccountSwitchesSignIn( t *testing.T ) {
	_ , store , _ := keyTestApp( t )
	owner , _ , _ := keyFor( t , store , "Owner" , models.RoleAdmin , false )
	_ , _ , key := keyFor( t , store , "Ana" , models.RoleUser , false )
	if _ , err := models.SetAPIKeySignIn( store , key.ID , owner.ID , true ); err == nil {
		t.Fatal( "the owner switched on someone else's key" )
	}
}

// The routes the project portal calls with the owner's admin key: Join makes
// an account and its link, Open reissues one, and matching lists accounts.
func TestTheOwnersAdminKeyServesThePortal( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	owner , admin_key , _ := keyFor( t , store , "Owner" , models.RoleAdmin , false )

	status , reply := callWithKey( t , app , admin_key , http.MethodPost , "/api/admin/users" ,
		map[string]any{ "display_name": "Cy" , "role": "user" , "max_uses": 1 , "expires_in_minutes": 15 } )
	if status != http.StatusCreated { t.Fatalf( "create user: %d %v" , status , reply ) }
	user_id , _ := reply[ "user_id" ].( float64 )
	if user_id == 0 || strings.HasPrefix( reply[ "login_path" ].( string ) , "/login/" ) == false {
		t.Fatalf( "create user answered %v" , reply )
	}

	status , reply = callWithKey( t , app , admin_key , http.MethodPost ,
		"/api/admin/users/" + jsonNumber( user_id ) + "/reissue-login" , portalLinkRequest )
	if status != http.StatusOK || strings.HasPrefix( reply[ "login_path" ].( string ) , "/login/" ) == false {
		t.Fatalf( "reissue: %d %v" , status , reply )
	}

	// Its own account's link is the Sign-in links switch's to give.
	status , _ = callWithKey( t , app , admin_key , http.MethodPost ,
		"/api/admin/users/" + jsonNumber( float64( owner.ID ) ) + "/reissue-login" , portalLinkRequest )
	if status != http.StatusForbidden { t.Fatalf( "a key reissued its own account's link: %d" , status ) }

	status , reply = callWithKey( t , app , admin_key , http.MethodGet , "/api/admin/users?limit=200&offset=0&q=cy" , nil )
	if status != http.StatusOK { t.Fatalf( "list users: %d %v" , status , reply ) }
	users , _ := reply[ "users" ].( []any )
	if len( users ) != 1 || reply[ "total" ].( float64 ) != 1 || reply[ "next" ] != nil {
		t.Fatalf( "list users answered %v" , reply )
	}
}

func jsonNumber( value float64 ) ( result string ) {
	encoded , _ := json.Marshal( value )
	result = string( encoded )
	return
}
