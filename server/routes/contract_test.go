package routes

import (
	http "net/http"
	testing "testing"
	time "time"

	db "rook/server/db"
	models "rook/server/models"
)

// Portal contract v2 (ProjectPortal's CROSS_PROJECT.md) as Rook speaks it:
// the viewer role, account expiry and key minting. Rook's owner is its
// superadmin, the oldest admin, so each test makes that account first.

// ownerKey makes the superadmin and an admin key of theirs, and records them
// as the superadmin the way the first owner-only request would.
func ownerKey( t *testing.T , store *db.Store ) ( owner *models.User , credential string , key *models.APIKey ) {
	t.Helper()
	owner , credential , key = keyFor( t , store , "Owner" , models.RoleAdmin , false )
	if models.IsSuperAdmin( store , owner.ID ) == false { t.Fatal( "the first admin is not the superadmin" ) }
	return
}

func TestMeSpeaksContractTwo( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	_ , credential , _ := ownerKey( t , store )
	status , reply := callWithKey( t , app , credential , http.MethodGet , "/api/me" , nil )
	if status != http.StatusOK { t.Fatalf( "me: %d %v" , status , reply ) }
	if reply[ "contract" ] != float64( 2 ) { t.Fatalf( "contract = %v, want 2" , reply[ "contract" ] ) }
	have := map[string]bool{}
	for _ , each := range reply[ "capabilities" ].( []any ) { have[ each.( string ) ] = true }
	for _ , want := range []string{ "viewer_role" , "account_expiry" , "key_minting" , "key_sign_in" } {
		if have[ want ] == false { t.Errorf( "capabilities %v lack %s" , reply[ "capabilities" ] , want ) }
	}
	if have[ "invite_links" ] { t.Errorf( "Rook has no invite links, but lists invite_links" ) }
}

func TestAViewerReadsButChangesNothing( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	ownerKey( t , store )
	_ , viewer , _ := keyFor( t , store , "Vera" , models.RoleViewer , false )

	status , reply := callWithKey( t , app , viewer , http.MethodGet , "/api/me" , nil )
	if status != http.StatusOK || reply[ "role" ] != models.RoleViewer {
		t.Fatalf( "a viewer reads /api/me: %d %v" , status , reply )
	}
	status , reply = callWithKey( t , app , viewer , http.MethodPost , "/api/account/avatar" , map[string]any{ "icon": "spade" } )
	if status != http.StatusForbidden || reply[ "error" ] != "view_only" {
		t.Fatalf( "a viewer changing its avatar: %d %v, want 403 view_only" , status , reply )
	}
}

func TestAnAccountExpires( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	owner , admin , _ := ownerKey( t , store )
	player , player_key , _ := keyFor( t , store , "Pat" , models.RoleUser , false )
	path := "/api/admin/users/" + jsonNumber( float64( player.ID ) ) + "/expiry"

	past := time.Now().Add( -time.Hour ).UTC().Format( time.RFC3339 )
	if status , reply := callWithKey( t , app , admin , http.MethodPost , path , map[string]any{ "expires_at": past } ); status != http.StatusBadRequest {
		t.Fatalf( "an expiry in the past: %d %v, want 400" , status , reply )
	}
	soon := time.Now().Add( time.Hour ).UTC().Format( time.RFC3339 )
	status , reply := callWithKey( t , app , admin , http.MethodPost , path , map[string]any{ "expires_at": soon } )
	if status != http.StatusOK || reply[ "expires_at" ] == nil { t.Fatalf( "set expiry: %d %v" , status , reply ) }

	owner_path := "/api/admin/users/" + jsonNumber( float64( owner.ID ) ) + "/expiry"
	if status , reply := callWithKey( t , app , admin , http.MethodPost , owner_path , map[string]any{ "expires_at": soon } ); status != http.StatusForbidden {
		t.Fatalf( "expiring the owner: %d %v, want 403" , status , reply )
	}

	// Once it passes, the account counts as disabled at once.
	stored , _ := models.GetUser( store , player.ID )
	gone := time.Now().Add( -time.Minute ).UTC()
	stored.ExpiresAt = &gone
	if err := models.SaveUser( store , stored ); err != nil { t.Fatal( err ) }
	if status , _ := callWithKey( t , app , player_key , http.MethodGet , "/api/me" , nil ); status != http.StatusUnauthorized {
		t.Fatalf( "an expired account's key: %d, want 401" , status )
	}
	_ , rows := callWithKey( t , app , admin , http.MethodGet , "/api/admin/users" , nil )
	for _ , each := range rows[ "users" ].( []any ) {
		row := each.( map[string]any )
		if row[ "id" ] == float64( player.ID ) && row[ "expired" ] != true { t.Fatalf( "user row %v is not expired" , row ) }
	}
}

func TestAKeyMintsKeysWithLineage( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	owner , admin , admin_key := ownerKey( t , store )
	player , _ , _ := keyFor( t , store , "Pat" , models.RoleUser , false )
	mint_path := "/api/admin/users/" + jsonNumber( float64( player.ID ) ) + "/keys"
	request := map[string]any{ "name": "portal" , "role": "admin" , "expires_in_days": 30 }

	if status , reply := callWithKey( t , app , admin , http.MethodPost , mint_path , request ); status != http.StatusForbidden || reply[ "error" ] != "key_minting_off" {
		t.Fatalf( "minting without the switch: %d %v, want 403 key_minting_off" , status , reply )
	}
	if _ , err := models.SetAPIKeyMinting( store , admin_key.ID , owner.ID , true ); err != nil { t.Fatal( err ) }

	status , reply := callWithKey( t , app , admin , http.MethodPost , mint_path , request )
	if status != http.StatusCreated { t.Fatalf( "mint: %d %v" , status , reply ) }
	if reply[ "role" ] != models.RoleUser { t.Fatalf( "role %v, want it capped at user" , reply[ "role" ] ) }
	if reply[ "minted_by_key" ] != admin_key.ID { t.Fatalf( "minted_by_key %v, want %s" , reply[ "minted_by_key" ] , admin_key.ID ) }
	minted , _ := reply[ "credential" ].( string )
	if status , _ := callWithKey( t , app , minted , http.MethodGet , "/api/me" , nil ); status != http.StatusOK {
		t.Fatalf( "the minted key: %d" , status )
	}
	minted_id , _ := reply[ "id" ].( string )
	if _ , err := models.SetAPIKeySignIn( store , minted_id , player.ID , true ); err == nil {
		t.Fatal( "a minted key got Sign-in links" )
	}

	_ , listed := callWithKey( t , app , admin , http.MethodGet , mint_path , nil )
	if keys , _ := listed[ "keys" ].( []any ); len( keys ) != 2 { t.Fatalf( "listed %v, want both of Pat's keys" , listed ) }

	owner_path := "/api/admin/users/" + jsonNumber( float64( owner.ID ) ) + "/keys"
	if status , reply := callWithKey( t , app , admin , http.MethodPost , owner_path , request ); status != http.StatusForbidden {
		t.Fatalf( "minting for the owner: %d %v, want 403" , status , reply )
	}

	// Revoking the key that made it revokes the minted one too.
	if err := models.RevokeAPIKey( store , admin_key.ID ); err != nil { t.Fatal( err ) }
	if status , _ := callWithKey( t , app , minted , http.MethodGet , "/api/me" , nil ); status != http.StatusUnauthorized {
		t.Fatalf( "the minted key after its maker was revoked: %d, want 401" , status )
	}
}

func TestOnlyTheOwnerLetsAKeyMint( t *testing.T ) {
	_ , store , _ := keyTestApp( t )
	ownerKey( t , store )
	other , _ , other_key := keyFor( t , store , "Ada" , models.RoleAdmin , false )
	if _ , err := models.SetAPIKeyMinting( store , other_key.ID , other.ID , true ); err == nil {
		t.Fatal( "an admin who is not the owner let a key mint" )
	}
}
