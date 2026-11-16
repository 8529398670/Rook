package routes

import (
	json "encoding/json"
	testing "testing"
)

// The lobby sends a new table's options as play_to alone, so the request
// check must not require the bird's rules (which the server sets itself).
func TestCreateTableOptionsAreOptional( t *testing.T ) {
	spec , _ := apiSpec( t )
	walk := func( from map[string]any , keys ...string ) map[string]any {
		for _ , key := range keys {
			next , ok := from[ key ].( map[string]any )
			if ok == false {
				t.Fatalf( "api spec has no %q on the way to POST /api/tables options" , key )
			}
			from = next
		}
		return from
	}
	options := walk( spec , "paths" , "/api/tables" , "post" , "requestBody" , "content" , "application/json" , "schema" , "properties" , "options" )
	if required , found := options[ "required" ]; found {
		t.Fatalf( "POST /api/tables options require %v; the lobby sends play_to alone" , required )
	}
}

// Leaving play_to out means the default; 0 is one hand, not the default.
func TestCreateTableOptionsPlayTo( t *testing.T ) {
	cases := map[string]int{
		`{}`:               500,
		`{"play_to":0}`:    0,
		`{"play_to":300}`:  300,
		`{"rook_rank":"low","play_to":100}`: 100,
	}
	for body , want := range cases {
		var sent createTableOptions
		if err := json.Unmarshal( []byte( body ) , &sent ); err != nil {
			t.Fatalf( "%s: %v" , body , err )
		}
		if got := sent.game().PlayTo; got != want {
			t.Fatalf( "%s: play_to %d, want %d" , body , got , want )
		}
	}
}
