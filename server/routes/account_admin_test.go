package routes

import (
	http "net/http"
	strings "strings"
	testing "testing"

	models "rook/server/models"
)

// A second admin -- the account the project portal works through -- runs
// other people's accounts the way the owner does, but never touches the
// owner's account or another admin's (RequireAccountAdmin). What is the
// owner's alone stays so.

func TestASecondAdminsKeyServesThePortal( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	ownerKey( t , store )
	_ , portal , _ := keyFor( t , store , "ProjectPortal" , models.RoleAdmin , false )

	status , reply := callWithKey( t , app , portal , http.MethodPost , "/api/admin/users" ,
		map[string]any{ "display_name": "Cy" , "role": "user" , "max_uses": 1 , "expires_in_minutes": 15 } )
	if status != http.StatusCreated { t.Fatalf( "create user: %d %v" , status , reply ) }
	user := "/api/admin/users/" + jsonNumber( reply[ "user_id" ].( float64 ) )

	status , reply = callWithKey( t , app , portal , http.MethodPost , user + "/reissue-login" , portalLinkRequest )
	if status != http.StatusOK || strings.HasPrefix( reply[ "login_path" ].( string ) , "/login/" ) == false {
		t.Fatalf( "reissue: %d %v" , status , reply )
	}
	if status , reply = callWithKey( t , app , portal , http.MethodGet , "/api/admin/users?limit=200&offset=0" , nil ); status != http.StatusOK {
		t.Fatalf( "list users: %d %v" , status , reply )
	}
	for _ , each := range []struct{ action string ; body map[string]any }{
		{ "role" , map[string]any{ "role": "viewer" } },
		{ "disabled" , map[string]any{ "disabled": true } },
		{ "disabled" , map[string]any{ "disabled": false } },
		{ "revoke-access" , map[string]any{} },
		{ "can-hint" , map[string]any{ "can_hint": true } },
		{ "delete" , map[string]any{} },
	} {
		if status , reply := callWithKey( t , app , portal , http.MethodPost , user + "/" + each.action , each.body ); status != http.StatusOK {
			t.Fatalf( "%s on a player: %d %v" , each.action , status , reply )
		}
	}
}

func TestASecondAdminLeavesTheOwnerAndOtherAdminsAlone( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	owner , owner_key , _ := ownerKey( t , store )
	_ , portal , _ := keyFor( t , store , "ProjectPortal" , models.RoleAdmin , false )
	peer , _ , _ := keyFor( t , store , "Pia" , models.RoleAdmin , false )

	for _ , target := range []*models.User{ owner , peer } {
		path := "/api/admin/users/" + jsonNumber( float64( target.ID ) )
		for action , body := range map[string]map[string]any{
			"reissue-login": portalLinkRequest,
			"revoke-access": {},
			"role":          { "role": "user" },
			"rename":        { "display_name": "X" },
			"can-undo":      { "can_undo": true },
			"disabled":      { "disabled": true },
		} {
			if status , reply := callWithKey( t , app , portal , http.MethodPost , path + "/" + action , body ); status != http.StatusForbidden {
				t.Fatalf( "a second admin's %s on %s: %d %v, want 403" , action , target.DisplayName , status , reply )
			}
		}
	}

	// The owner still runs other admins' accounts.
	status , reply := callWithKey( t , app , owner_key , http.MethodPost ,
		"/api/admin/users/" + jsonNumber( float64( peer.ID ) ) + "/reissue-login" , portalLinkRequest )
	if status != http.StatusOK { t.Fatalf( "the owner reissuing an admin's link: %d %v" , status , reply ) }

	// Game settings and the account keys stay the owner's.
	if status , reply := callWithKey( t , app , portal , http.MethodGet , "/api/admin/settings" , nil ); status != http.StatusForbidden {
		t.Fatalf( "a second admin reading game settings: %d %v, want 403" , status , reply )
	}
	player , _ , _ := keyFor( t , store , "Pat" , models.RoleUser , false )
	if status , reply := callWithKey( t , app , portal , http.MethodGet , "/api/admin/users/" + jsonNumber( float64( player.ID ) ) + "/keys" , nil ); status != http.StatusForbidden {
		t.Fatalf( "a second admin listing a player's keys: %d %v, want 403" , status , reply )
	}
}

func TestAPlayerIsNoAccountAdmin( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	ownerKey( t , store )
	_ , player , _ := keyFor( t , store , "Pat" , models.RoleUser , false )
	if status , reply := callWithKey( t , app , player , http.MethodGet , "/api/admin/users" , nil ); status != http.StatusForbidden {
		t.Fatalf( "a player listing accounts: %d %v, want 403" , status , reply )
	}
}
