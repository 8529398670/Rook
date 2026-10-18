package models

import (
	errors "errors"
	testing "testing"
	time "time"

	config "rook/server/config"
	db "rook/server/db"
)

func testStore( t *testing.T ) ( store *db.Store ) {
	t.Helper()
	t.Setenv( "APP_DIR" , t.TempDir() )
	cfg , err := config.Load()
	if err != nil { t.Fatal( err ) }
	store , err = db.Open( cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( func() { store.Close() } )
	return
}

func TestNameCloseness( t *testing.T ) {
	type pair struct {
		typed, existing string
		similar         bool
	}
	pairs := []pair{
		{ "Ana" , "ana" , true },
		{ "anna marie" , "Anna-Marie" , true },
		{ "Ann" , "Anna" , true },          // prefix
		{ "Jonathon" , "Jonathan" , true }, // one letter off
		{ "Katherine" , "Catherine" , true },
		{ "Bob" , "Rob" , true },
		{ "Bob" , "Sue" , false },
		{ "Al" , "Alexander" , false }, // too short to suggest by prefix
		{ "Michael" , "Rachel" , false },
	}
	for _ , item := range pairs {
		if got := NameCloseness( item.typed , item.existing ) >= 0; got != item.similar {
			t.Errorf( "%q vs %q: similar=%v, want %v" , item.typed , item.existing , got , item.similar )
		}
	}
	if NameCloseness( "ana" , "Ana" ) != 0 {
		t.Errorf( "an exact match should score 0" )
	}
}

func TestSignupLinkCapacityAndMatches( t *testing.T ) {
	store := testStore( t )
	credential , link , err := IssueSignupLink( store , "Family" , 2 , time.Hour , 1 )
	if err != nil { t.Fatal( err ) }

	if _ , err := LoadSignupLink( store , link.ID+".wrong" ); !errors.Is( err , ErrSignupLinkInvalid ) {
		t.Fatalf( "wrong secret accepted: %v" , err )
	}
	loaded , err := LoadSignupLink( store , credential )
	if err != nil || loaded.Remaining() != 2 { t.Fatalf( "load: %v" , err ) }

	ana , _ := CreateUser( store , "Ana" , RoleUser )
	ben , _ := CreateUser( store , "Ben" , RoleUser )
	outsider , _ := CreateUser( store , "Anabel" , RoleUser )
	if err := RecordSignup( store , link.ID , ana.ID ); err != nil { t.Fatal( err ) }
	if err := RecordSignup( store , link.ID , ben.ID ); err != nil { t.Fatal( err ) }
	if err := RecordSignup( store , link.ID , outsider.ID ); !errors.Is( err , ErrSignupLinkFull ) {
		t.Fatalf( "third sign-up on a two-place link: %v" , err )
	}

	loaded , _ = LoadSignupLink( store , credential )
	matches , _ := SignupMatches( store , loaded , "anna" )
	if len( matches ) != 1 || matches[ 0 ].ID != ana.ID {
		t.Fatalf( "matches for anna: %v (the outsider Anabel must never be offered)" , matches )
	}

	RevokeSignupLink( store , link.ID )
	if _ , err := LoadSignupLink( store , credential ); !errors.Is( err , ErrSignupLinkInvalid ) {
		t.Fatalf( "revoked link still loads" )
	}
}

func TestPin( t *testing.T ) {
	store := testStore( t )
	user , _ := CreateUser( store , "Ana" , RoleUser )
	if err := SetUserPin( store , user.ID , "12a4" ); !errors.Is( err , ErrBadPin ) {
		t.Fatalf( "letters in a pin: %v" , err )
	}
	if err := SetUserPin( store , user.ID , "4321" ); err != nil { t.Fatal( err ) }
	user , _ = GetUser( store , user.ID )
	if user.HasPin() == false || user.CheckPin( "4321" ) == false || user.CheckPin( "1234" ) || user.CheckPin( "" ) {
		t.Fatalf( "pin check wrong" )
	}
	SetUserPin( store , user.ID , "" )
	user , _ = GetUser( store , user.ID )
	if user.HasPin() || user.CheckPin( "" ) == false {
		t.Fatalf( "clearing the pin did not work" )
	}
}
