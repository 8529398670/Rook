package models

import (
	errors "errors"
	sync "sync"
	testing "testing"
	time "time"

	db "rook/server/db"
)

func countFor( store *db.Store , bucket string , newItem func() any , match func( any ) bool ) ( count int ) {
	store.ForEach( bucket , newItem , func( key []byte , item any ) bool {
		if match( item ) { count++ }
		return true
	} )
	return
}

func sessionsOf( store *db.Store , user_id uint64 ) int {
	return countFor( store , db.BucketSessions , func() any { return &Session{} } ,
		func( item any ) bool { return item.( *Session ).UserID == user_id } )
}

func TestRedeemLoginTokenMakesOneSessionOnce( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	link , err := IssueLoginToken( store , ana.ID , time.Hour )
	if err != nil { t.Fatal( err ) }

	// Racing the same link: exactly one request may get a session.
	var wins sync.WaitGroup
	results := make( chan error , 8 )
	for range 8 {
		wins.Add( 1 )
		go func() {
			defer wins.Done()
			_ , _ , redeem_err := RedeemLoginToken( store , link , time.Hour )
			results <- redeem_err
		}()
	}
	wins.Wait()
	close( results )
	ok := 0
	for result := range results {
		if result == nil {
			ok++
		} else if errors.Is( result , ErrLoginTokenInvalid ) == false {
			t.Fatalf( "unexpected error: %v" , result )
		}
	}
	if ok != 1 { t.Fatalf( "%d redemptions succeeded, want 1" , ok ) }
	if got := sessionsOf( store , ana.ID ); got != 1 { t.Fatalf( "%d sessions, want 1" , got ) }

	credential , user_id , err := RedeemLoginToken( store , link , time.Hour )
	if errors.Is( err , ErrLoginTokenInvalid ) == false || credential != "" || user_id != 0 {
		t.Fatalf( "spent link redeemed again: %q %d %v" , credential , user_id , err )
	}
}

func TestRedeemLoginTokenSessionWorks( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	link , _ := IssueLoginToken( store , ana.ID , time.Hour )
	credential , user_id , err := RedeemLoginToken( store , link , time.Hour )
	if err != nil || user_id != ana.ID { t.Fatalf( "redeem: %d %v" , user_id , err ) }
	if _ , user := LoadSession( store , credential ); user == nil || user.ID != ana.ID {
		t.Fatalf( "session from the link does not load" )
	}
}

func TestNewLoginLinkRetiresTheOldOne( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	old , _ := IssueLoginToken( store , ana.ID , time.Hour )
	fresh , _ := IssueLoginToken( store , ana.ID , time.Hour )
	if _ , _ , err := RedeemLoginToken( store , old , time.Hour ); errors.Is( err , ErrLoginTokenInvalid ) == false {
		t.Fatalf( "replaced link still works: %v" , err )
	}
	if _ , _ , err := RedeemLoginToken( store , fresh , time.Hour ); err != nil {
		t.Fatalf( "new link refused: %v" , err )
	}
}

func TestDisabledAccountCannotSignIn( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	link , _ := IssueLoginToken( store , ana.ID , time.Hour )
	if _ , _ , err := CreateSession( store , ana.ID , time.Hour ); err != nil { t.Fatal( err ) }

	if err := SetUserDisabled( store , ana.ID , true ); err != nil { t.Fatal( err ) }
	if got := sessionsOf( store , ana.ID ); got != 0 { t.Fatalf( "%d sessions survived disabling" , got ) }
	if _ , _ , err := CreateSession( store , ana.ID , time.Hour ); errors.Is( err , ErrAccountDisabled ) == false {
		t.Fatalf( "session created for a disabled account: %v" , err )
	}

	// The link was deleted with the sessions, so re-enabling does not bring
	// it back to life.
	if err := SetUserDisabled( store , ana.ID , false ); err != nil { t.Fatal( err ) }
	if _ , _ , err := RedeemLoginToken( store , link , time.Hour ); errors.Is( err , ErrLoginTokenInvalid ) == false {
		t.Fatalf( "link issued before disabling works after re-enabling: %v" , err )
	}
}

func TestRevokeAllCredentials( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	ben , _ := CreateUser( store , "Ben" , RoleUser )
	for _ , user := range []*User{ ana , ben } {
		CreateSession( store , user.ID , time.Hour )
		CreateSession( store , user.ID , time.Hour )
		IssueLoginToken( store , user.ID , time.Hour )
		IssueAPIKey( store , user.ID , "script" , RoleUser , time.Hour )
	}
	ana_key , _ , _ := IssueAPIKey( store , ana.ID , "other" , RoleUser , time.Hour )

	revoked , err := RevokeAllCredentials( store , ana.ID )
	if err != nil { t.Fatal( err ) }
	if revoked != ( Revoked{ Sessions: 2 , LoginLinks: 1 , APIKeys: 2 } ) { t.Fatalf( "revoked %+v" , revoked ) }
	if _ , user := LoadAPIKey( store , ana_key ); user != nil { t.Fatal( "revoked key still authenticates" ) }
	if got := sessionsOf( store , ana.ID ); got != 0 { t.Fatalf( "%d sessions left" , got ) }
	if got := sessionsOf( store , ben.ID ); got != 2 { t.Fatalf( "someone else's sessions touched: %d" , got ) }
	if user , _ := GetUser( store , ana.ID ); user.Disabled() { t.Fatal( "revoking disabled the account" ) }

	if _ , err := RevokeAllCredentials( store , 9999 ); errors.Is( err , db.ErrNotFound ) == false {
		t.Fatalf( "unknown account: %v" , err )
	}
}

func TestAPIKeyQuotaHoldsUnderConcurrency( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	for range APIKeyMaxPerUser - 1 {
		if _ , _ , err := IssueAPIKey( store , ana.ID , "k" , RoleUser , time.Hour ); err != nil { t.Fatal( err ) }
	}
	var group sync.WaitGroup
	for range 16 {
		group.Add( 1 )
		go func() {
			defer group.Done()
			IssueAPIKey( store , ana.ID , "race" , RoleUser , time.Hour )
		}()
	}
	group.Wait()
	live , _ := CountLiveAPIKeysForUser( store , ana.ID )
	if live != APIKeyMaxPerUser { t.Fatalf( "%d live keys, cap is %d" , live , APIKeyMaxPerUser ) }
}

func TestAPIKeyRowsStayBounded( t *testing.T ) {
	store := testStore( t )
	ana , _ := CreateUser( store , "Ana" , RoleUser )
	for range apiKeyMaxRowsPerUser + 20 {
		_ , key , err := IssueAPIKey( store , ana.ID , "k" , RoleUser , time.Hour )
		if err != nil { t.Fatal( err ) }
		if err = RevokeAPIKey( store , key.ID ); err != nil { t.Fatal( err ) }
	}
	keys , _ := ListAPIKeysForUser( store , ana.ID )
	if len( keys ) > apiKeyMaxRowsPerUser { t.Fatalf( "%d rows kept, limit %d" , len( keys ) , apiKeyMaxRowsPerUser ) }
}
