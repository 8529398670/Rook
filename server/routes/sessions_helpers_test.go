package routes

import (
	encoding_json "encoding/json"
	io "io"
	http "net/http"
	strings "strings"
	testing "testing"
	time "time"

	fiber "github.com/gofiber/fiber/v3"
)

// devices is a browser's GET /api/account/sessions: the rows of its first page.
// The template keeps it in sessions_test.go, which Rook does not carry; the
// portal's admin_sessions test (portal_test.go) needs it.
func ( client *browser ) devices( t *testing.T ) ( rows []map[ string ]any ) {
	t.Helper()
	request , _ := http.NewRequest( http.MethodGet , "/api/account/sessions" , nil )
	request.Header.Set( "Cookie" , client.cookie )
	request.Header.Set( "User-Agent" , "test-agent" )
	response , err := client.fixture.app.Test( request , fiber.TestConfig{ Timeout: 10 * time.Second } )
	if err != nil { t.Fatal( err ) }
	raw , _ := io.ReadAll( response.Body )
	if response.StatusCode != http.StatusOK {
		t.Fatalf( "status %d: %s" , response.StatusCode , raw )
	}
	if strings.Contains( string( raw ) , "secret_hash" ) || strings.Contains( string( raw ) , "csrf" ) {
		t.Fatalf( "the list leaks a credential: %s" , raw )
	}
	var page struct {
		Sessions []map[ string ]any `json:"sessions"`
	}
	if err = encoding_json.Unmarshal( raw , &page ); err != nil { t.Fatal( err ) }
	rows = page.Sessions
	return
}
