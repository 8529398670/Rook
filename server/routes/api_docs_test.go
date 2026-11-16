package routes

import (
	bytes "bytes"
	flag "flag"
	http "net/http"
	os "os"
	filepath "path/filepath"
	strings "strings"
	sync "sync"
	testing "testing"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
	routegen "rook/server/apidoc/routegen"
	security "rook/server/security"
)

// The API reference is written from the code: config/api.json and
// config/api.md from each route's declaration (the // @route(...) at the
// end of its handler's doc comment), and config/mcp.json and config/mcp.md from the registered MCP tools.
// scripts/api_docs.sh runs TestAPIDocsAreCurrent with -update to rewrite
// them; without it, the test fails while a committed file is out of date.
var updateAPIDocs = flag.Bool( "update" , false , "rewrite config/api.json, config/mcp.json, config/api.md and config/mcp.md from the code" )

// notDocumented is what the route table has that is not an API: the pages a
// link opens, and the catch-alls.
var notDocumented = map[string]bool{
	"GET /login/*": true,
	"GET /docs/*":  true,
	"GET /join/*":  true,
	"GET /api/*":   true,
	"GET /*":       true,
}

// docsApp is the app's route table for the docs: Register on handlers that
// are never called, so nothing behind them (the store, the tables) needs to
// exist. Rook has no switches that hide routes from it.
type docsApp struct {
	app *fiber.App
}

func newParityFixture( t *testing.T ) ( fixture *docsApp ) {
	t.Helper()
	handlers := &Handlers{ Guard: &security.Guard{} }
	app := fiber.New()
	handlers.Register( app )
	fixture = &docsApp{ app: app }
	return
}

// apiSpec is api.json as the code describes it, built from the route
// table of the app with everything on. A route mounted without a
// declaration fails the test that asked.
func apiSpec( t *testing.T ) ( spec map[string]any , fixture *docsApp ) {
	t.Helper()
	fixture = newParityFixture( t )
	routes := documentedRoutes( fixture.app )
	built , problems := apidoc.Build( routes )
	if len( problems ) > 0 { t.Fatal( strings.Join( problems , "\n" ) ) }
	spec , err := apidoc.Plain( built )
	if err != nil { t.Fatal( err ) }
	return
}

// documentedRoutes is the app's route table, less what is not an API.
func documentedRoutes( app *fiber.App ) ( routes []apidoc.Route ) {
	for _ , route := range app.GetRoutes( true ) {
		if route.Method != http.MethodGet && route.Method != http.MethodPost && route.Method != http.MethodDelete { continue }
		if route.Path == "/fail" || route.Path == "/panic" { continue } // newMonitored's own
		if notDocumented[ route.Method + " " + route.Path ] { continue }
		routes = append( routes , apidoc.Route{ Method: route.Method , Path: route.Path , Name: route.Name } )
	}
	return
}

// checkReplies holds every success reply a test gets against the API docs,
// so docs that say something a handler does not do fail a test. Every test
// app has it in front of its routes.
func checkReplies( t *testing.T ) ( handler fiber.Handler ) {
	var once sync.Once
	var spec map[string]any
	handler = func( c fiber.Ctx ) ( err error ) {
		err = c.Next()
		status := c.Response().StatusCode()
		if err != nil || status < 200 || status > 299 { return }
		if strings.HasPrefix( string( c.Response().Header.ContentType() ) , fiber.MIMEApplicationJSON ) == false { return }
		once.Do( func() {
			built , _ := apidoc.Build( documentedRoutes( c.App() ) )
			spec , _ = apidoc.Plain( built )
		} )
		route := c.Route()
		path := apidoc.SpecPath( route.Path )
		for _ , problem := range apidoc.Check( spec , route.Method , path , status , c.Response().Body() ) {
			t.Errorf( "%s %s: %s" , route.Method , path , problem )
		}
		return
	}
	return
}

// Every route is declared.
func TestEveryRouteIsDocumented( t *testing.T ) {
	apiSpec( t )
}

// The route table is what apidoc/gen writes from the handlers' @route(...)
// declarations. scripts/api_docs.sh rewrites it (go generate).
func TestTheRouteTableIsCurrent( t *testing.T ) {
	want , err := routegen.Generate( "." )
	if err != nil { t.Fatal( err ) }
	have , err := os.ReadFile( routegen.FileName )
	if err != nil || bytes.Equal( have , want ) == false {
		t.Errorf( "server/routes/%s is out of date: run scripts/api_docs.sh" , routegen.FileName )
	}
}

func TestAPIDocsAreCurrent( t *testing.T ) {
	spec , _ := apiSpec( t )
	files := map[string][]byte{}
	var err error
	var manifest map[string]any
	if files[ "api.json" ] , err = apidoc.Encode( spec ); err != nil { t.Fatal( err ) }
	files[ "api.md" ] = apidoc.Markdown( spec , manifest )
	for name , want := range files {
		path := filepath.Join( "../../config" , name )
		if *updateAPIDocs {
			if err = os.WriteFile( path , want , 0o644 ); err != nil { t.Fatal( err ) }
			continue
		}
		have , read_err := os.ReadFile( path )
		if read_err != nil || bytes.Equal( have , want ) == false {
			t.Errorf( "config/%s is out of date: run scripts/api_docs.sh" , name )
		}
	}
}
