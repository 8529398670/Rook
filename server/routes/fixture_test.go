package routes

import (
	json "encoding/json"
	io "io"
	http "net/http"
	strconv "strconv"
	strings "strings"
	testing "testing"
	fstest "testing/fstest"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	config "rook/server/config"
	db "rook/server/db"
	language "rook/server/language"
	logs "rook/server/logs"
	models "rook/server/models"
	security "rook/server/security"
	static "rook/server/static"
	tables "rook/server/tables"
)

// The template's shared test fixture, as Rook builds its app: what the
// portal contract tests (portal_conformance_test.go, portal_test.go,
// key_rotation_test.go, error_codes_test.go, actor_audit_test.go) drive the
// routes with. The owner is Rook's superadmin, the first admin made, so it
// is made first and recorded at once.

type authFixture struct {
	app      *fiber.App
	store    *db.Store
	cfg      *config.Config
	owner    *models.User
	alice    *models.User
	handlers *Handlers
	// userKey is an ordinary user's key (alice's).
	userKey string
}

// browser is one cookie jar.
type browser struct {
	fixture *authFixture
	cookie  string
}

func newAuthFixture( t *testing.T ) ( fixture *authFixture ) {
	t.Helper()
	cfg , err := config.ForTest( t.TempDir() )
	if err != nil { t.Fatal( err ) }
	cfg.APIKeysEnabled = true
	cfg.SessionTTL = time.Hour
	store , err := db.Open( cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( func() { store.Close() } )
	hub , err := tables.New( store , cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( hub.Close )

	owner , err := models.CreateUser( store , "Owner" , models.RoleAdmin )
	if err != nil { t.Fatal( err ) }
	if models.IsSuperAdmin( store , owner.ID ) == false { t.Fatal( "the first admin is not the superadmin" ) }
	alice , err := models.CreateUser( store , "Alice" , models.RoleUser )
	if err != nil { t.Fatal( err ) }
	user_key , _ , err := models.IssueAPIKey( store , alice.ID , "script" , models.RoleUser , time.Hour )
	if err != nil { t.Fatal( err ) }

	lang , err := language.Parse( []byte( "app:\n  title: \"Test\"\n" ) )
	if err != nil { t.Fatal( err ) }
	static_server , err := static.New( fstest.MapFS{ "index.html": { Data: []byte( "index" ) } } )
	if err != nil { t.Fatal( err ) }
	handlers := New( store , cfg , security.New( store , cfg ) , lang , static_server , hub )
	// The activity log on, as in a real install, so its routes are there.
	logger := logs.New( config.LogsConfig{ Enabled: true , Dir: t.TempDir() , RetentionDays: 7 , MaxTotalBytes: 1 << 20 , MaxFileBytes: 1 << 20 } , cfg.SecretKey , false )
	logger.Start()
	t.Cleanup( logger.Close )
	handlers.Logs = logger
	app := fiber.New()
	handlers.Register( app )
	fixture = &authFixture{ app: app , store: store , cfg: cfg , owner: owner , alice: alice , handlers: handlers , userKey: user_key }
	return
}

func idText( id uint64 ) ( result string ) {
	result = strconv.FormatUint( id , 10 )
	return
}

func ( fixture *authFixture ) browser() ( result *browser ) {
	result = &browser{ fixture: fixture }
	return
}

func ( client *browser ) do( t *testing.T , method string , path string , body fiber.Map ) ( status int , reply map[ string ]any ) {
	t.Helper()
	raw , _ := json.Marshal( body )
	request , _ := http.NewRequest( method , path , strings.NewReader( string( raw ) ) )
	request.Header.Set( "Content-Type" , "application/json" )
	if client.cookie != "" {
		request.Header.Set( "Cookie" , client.cookie )
	}
	response , err := client.fixture.app.Test( request , fiber.TestConfig{ Timeout: 10 * time.Second } )
	if err != nil { t.Fatal( err ) }
	for _ , cookie := range response.Cookies() {
		client.cookie = cookie.Name + "=" + cookie.Value
	}
	text , _ := io.ReadAll( response.Body )
	status = response.StatusCode
	json.Unmarshal( text , &reply )
	return
}

func ( client *browser ) post( t *testing.T , path string , body fiber.Map ) ( status int , reply map[ string ]any ) {
	t.Helper()
	status , reply = client.do( t , http.MethodPost , path , body )
	return
}

// send is a POST with the session's csrf token.
func send( t *testing.T , client *browser , path string , body fiber.Map ) ( status int , reply map[ string ]any ) {
	t.Helper()
	body[ "csrf_token" ] = client.csrf( t )
	status , reply = client.post( t , path , body )
	return
}

func ( client *browser ) csrf( t *testing.T ) ( token string ) {
	t.Helper()
	_ , reply := client.do( t , http.MethodGet , "/api/me" , nil )
	token , _ = reply[ "csrf_token" ].( string )
	return
}

// withKey calls a route the way a script does: an API key in the
// Authorization header, no cookie and no csrf token.
func ( fixture *authFixture ) withKey( t *testing.T , key string , method string , path string , body fiber.Map ) ( status int , reply map[ string ]any ) {
	t.Helper()
	raw , _ := json.Marshal( body )
	request , _ := http.NewRequest( method , path , strings.NewReader( string( raw ) ) )
	request.Header.Set( "Content-Type" , "application/json" )
	request.Header.Set( "Authorization" , "Bearer " + key )
	response , err := fixture.app.Test( request , fiber.TestConfig{ Timeout: 10 * time.Second } )
	if err != nil { t.Fatal( err ) }
	text , _ := io.ReadAll( response.Body )
	status = response.StatusCode
	json.Unmarshal( text , &reply )
	return
}

func ( fixture *authFixture ) signIn( t *testing.T , user_id uint64 ) ( client *browser ) {
	t.Helper()
	credential , err := models.IssueLoginToken( fixture.store , user_id , time.Hour )
	if err != nil { t.Fatal( err ) }
	client = fixture.browser()
	if status , reply := client.post( t , "/api/login" , fiber.Map{ "credential": credential } ); status != 200 {
		t.Fatalf( "login link: %d %v" , status , reply )
	}
	return
}

// monitored is authFixture with the activity log on, writing to logDir.
type monitored struct {
	*authFixture
	logDir string
	logger *logs.Logger
}

func newMonitored( t *testing.T , logging bool ) ( fixture *monitored ) {
	t.Helper()
	base := newAuthFixture( t )
	dir := t.TempDir()
	logger := logs.New( config.LogsConfig{ Enabled: logging , Dir: dir , RetentionDays: 7 , MaxTotalBytes: 1 << 20 , MaxFileBytes: 1 << 20 } , base.cfg.SecretKey , false )
	logger.Start()
	base.handlers.Logs = logger
	fixture = &monitored{ authFixture: base , logDir: dir , logger: logger }
	return
}
