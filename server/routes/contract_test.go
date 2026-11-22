package routes

import (
	http "net/http"
	testing "testing"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	db "rook/server/db"
	models "rook/server/models"
)

// The portal contract (the go-webserver skill's SKILL.md, v3) as Rook speaks
// it: the viewer role, account expiry, key minting, and v3's service
// accounts, key grants, error codes and key rotation. Rook's owner is its
// superadmin, the oldest admin, so each test makes that account first.

// ownerKey makes the superadmin and an admin key of theirs, and records them
// as the superadmin the way the first owner-only request would.
func ownerKey( t *testing.T , store *db.Store ) ( owner *models.User , credential string , key *models.APIKey ) {
	t.Helper()
	owner , credential , key = keyFor( t , store , "Owner" , models.RoleAdmin , false )
	if models.IsSuperAdmin( store , owner.ID ) == false { t.Fatal( "the first admin is not the superadmin" ) }
	return
}

func TestMeSpeaksContractThree( t *testing.T ) {
	app , store , _ := keyTestApp( t )
	_ , credential , _ := ownerKey( t , store )
	status , reply := callWithKey( t , app , credential , http.MethodGet , "/api/me" , nil )
	if status != http.StatusOK { t.Fatalf( "me: %d %v" , status , reply ) }
	if reply[ "contract" ] != float64( 3 ) { t.Fatalf( "contract = %v, want 3" , reply[ "contract" ] ) }
	have := map[string]bool{}
	for _ , each := range reply[ "capabilities" ].( []any ) { have[ each.( string ) ] = true }
	for _ , want := range []string{ "viewer_role" , "account_expiry" , "key_minting" , "key_sign_in" , "service_accounts" , "key_grants" , "error_codes" , "key_rotation" } {
		if have[ want ] == false { t.Errorf( "capabilities %v lack %s" , reply[ "capabilities" ] , want ) }
	}
	if have[ "invite_links" ] { t.Errorf( "Rook has no invite links, but lists invite_links" ) }
}

// /api/me says what this caller may do, not only what the app supports
// (portal contract v3, item 3): owner and kind always, and for a key, the
// key's own grants and expiry.
func TestMeSaysWhatThisCallerMayDo( t *testing.T ) {
	fixture := newAuthFixture( t )
	owner := fixture.signIn( t , fixture.owner.ID )
	status , me := owner.do( t , http.MethodGet , "/api/me" , nil )
	if status != fiber.StatusOK || me[ "owner" ] != true || me[ "kind" ] != "person" { t.Fatalf( "%d %v" , status , me ) }
	if _ , has := me[ "key" ]; has { t.Fatalf( "a session has no key: %v" , me ) }
	alice := fixture.signIn( t , fixture.alice.ID )
	if _ , me = alice.do( t , http.MethodGet , "/api/me" , nil ); me[ "owner" ] != false || me[ "kind" ] != "person" { t.Fatalf( "%v" , me ) }

	credential , key , err := models.IssueAPIKey( fixture.store , fixture.owner.ID , "portal" , models.RoleAdmin , 12 * 24 * time.Hour )
	if err != nil { t.Fatal( err ) }
	status , me = fixture.withKey( t , credential , http.MethodGet , "/api/me" , nil )
	if status != fiber.StatusOK || me[ "owner" ] != true || me[ "kind" ] != "person" { t.Fatalf( "%d %v" , status , me ) }
	grants , _ := me[ "key" ].( map[ string ]any )
	if grants == nil || grants[ "id" ] != key.ID || grants[ "name" ] != "portal" || grants[ "role" ] != "admin" ||
		grants[ "may_mint_keys" ] != false || grants[ "may_sign_in" ] != false || grants[ "expires_at" ] != key.ExpiresAt.UTC().Format( time.RFC3339Nano ) {
		t.Fatalf( "key: %v (want expiry %v)" , grants , key.ExpiresAt )
	}
	// A key that never expires says so by leaving expires_at out.
	forever , _ , err := models.IssueAPIKey( fixture.store , fixture.alice.ID , "cron" , models.RoleUser , 0 )
	if err != nil { t.Fatal( err ) }
	_ , me = fixture.withKey( t , forever , http.MethodGet , "/api/me" , nil )
	if grants , _ = me[ "key" ].( map[ string ]any ); grants == nil || me[ "owner" ] != false { t.Fatalf( "%v" , me ) }
	if _ , has := grants[ "expires_at" ]; has { t.Fatalf( "never expires: %v" , grants ) }
	listed := false
	for _ , name := range me[ "capabilities" ].( []any ) { listed = listed || name == "key_grants" }
	if listed == false { t.Fatalf( "key_grants missing: %v" , me[ "capabilities" ] ) }
}

// /api/me says which skill and commit the app was scaffolded from (portal
// contract v3, item 8), so the portal can say when it is behind. The build
// stamps them from .skill-origin.yaml; without it, both are left out.
func TestMeSaysWhichTemplateItCameFrom( t *testing.T ) {
	fixture := newAuthFixture( t )
	alice := fixture.signIn( t , fixture.alice.ID )
	_ , me := alice.do( t , http.MethodGet , "/api/me" , nil )
	for _ , name := range []string{ "skill" , "skill_commit" } {
		if _ , has := me[ name ]; has { t.Fatalf( "%s without a stamp: %v" , name , me ) }
	}
	fixture.handlers.Skill , fixture.handlers.SkillCommit = "go-webserver" , "ea9f39c0123456789"
	_ , me = alice.do( t , http.MethodGet , "/api/me" , nil )
	if me[ "skill" ] != "go-webserver" || me[ "skill_commit" ] != "ea9f39c0123456789" { t.Fatalf( "%v" , me ) }
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

// The portal works through an admin account of its own, never the owner's
// (portal contract v3, item 1). Every account route lets any admin run an
// ordinary account, and keeps to the ladder: nobody acts on the owner's
// account, and only the owner acts on another admin's.
func TestASecondAdminRunsAccountsByTheLadder( t *testing.T ) {
	fixture := newAuthFixture( t )
	portal_user , _ := models.CreateUser( fixture.store , "ProjectPortal" , models.RoleAdmin )
	peer , _ := models.CreateUser( fixture.store , "Pia" , models.RoleAdmin )
	portal , _ , err := models.IssueAPIKey( fixture.store , portal_user.ID , "portal" , models.RoleAdmin , 0 )
	if err != nil { t.Fatal( err ) }

	status , reply := fixture.withKey( t , portal , http.MethodPost , "/api/admin/users" , fiber.Map{ "display_name": "Cy" , "role": "user" , "max_uses": 1 , "expires_in_minutes": 15 } )
	if status != fiber.StatusCreated { t.Fatalf( "create: %d %v" , status , reply ) }
	cy := "/api/admin/users/" + idText( uint64( reply[ "user_id" ].( float64 ) ) )
	if status , reply = fixture.withKey( t , portal , http.MethodGet , "/api/admin/users?limit=1" , nil ); status != fiber.StatusOK { t.Fatalf( "list: %d %v" , status , reply ) }
	soon := time.Now().UTC().Add( time.Hour ).Format( time.RFC3339 )
	for _ , each := range []struct{ action string ; body fiber.Map }{
		{ "reissue-login" , fiber.Map{ "max_uses": 1 , "expires_in_minutes": 15 } },
		{ "role" , fiber.Map{ "role": "viewer" } },
		{ "expiry" , fiber.Map{ "expires_at": soon } },
		{ "disabled" , fiber.Map{ "disabled": true } },
		{ "disabled" , fiber.Map{ "disabled": false } },
		{ "revoke-access" , fiber.Map{} },
		{ "delete" , fiber.Map{} },
	} {
		if status , reply := fixture.withKey( t , portal , http.MethodPost , cy + "/" + each.action , each.body ); status != fiber.StatusOK {
			t.Fatalf( "%s on an ordinary account: %d %v" , each.action , status , reply )
		}
	}

	// A session of the same admin may list keys; the ladder still holds.
	admin := fixture.signIn( t , portal_user.ID )
	if status , reply := admin.do( t , http.MethodGet , "/api/admin/users/" + idText( fixture.alice.ID ) + "/keys" , nil ); status != fiber.StatusOK {
		t.Fatalf( "keys of an ordinary account: %d %v" , status , reply )
	}
	for _ , target := range []*models.User{ fixture.owner , peer } {
		path := "/api/admin/users/" + idText( target.ID )
		if status , reply := admin.do( t , http.MethodGet , path + "/keys" , nil ); status != fiber.StatusForbidden {
			t.Fatalf( "a second admin listing %s's keys: %d %v, want 403" , target.DisplayName , status , reply )
		}
		for action , body := range map[ string ]fiber.Map{
			"reissue-login": { "max_uses": 1 , "expires_in_minutes": 15 },
			"role":          { "role": "user" },
			"expiry":        { "expires_at": soon },
			"disabled":      { "disabled": true },
			"revoke-access": {},
			"delete":        {},
		} {
			if status , reply := fixture.withKey( t , portal , http.MethodPost , path + "/" + action , body ); status != fiber.StatusForbidden {
				t.Fatalf( "a second admin's %s on %s: %d %v, want 403" , action , target.DisplayName , status , reply )
			}
		}
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
