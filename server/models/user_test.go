package models

import (
	errors "errors"
	testing "testing"
	time "time"

	db "rook/server/db"
)

func TestValidColor( t *testing.T ) {
	for _ , color := range []string{ "" , "teal" , "berry" , "h0" , "h7" , "h217" , "h359" } {
		if ValidColor( color ) == false { t.Fatalf( "%q should be allowed" , color ) }
	}
	for _ , color := range []string{ "h360" , "h-1" , "h007" , "h" , "H20" , "#ff0000" , "teal " , "h12x" } {
		if ValidColor( color ) { t.Fatalf( "%q should be refused" , color ) }
	}
}

func TestDeleteUserRemovesEveryWayIn( t *testing.T ) {
	store := testStore( t )
	doomed , _ := CreateUser( store , "Ana" , RoleUser )
	kept , _ := CreateUser( store , "Ben" , RoleUser )
	for _ , user := range []*User{ doomed , kept } {
		if _ , _ , err := CreateSession( store , user.ID , time.Hour ); err != nil { t.Fatal( err ) }
		if _ , err := IssueLoginToken( store , user.ID , time.Hour ); err != nil { t.Fatal( err ) }
		if _ , _ , err := IssueAPIKey( store , user.ID , "script" , RoleUser , time.Hour ); err != nil { t.Fatal( err ) }
	}

	if err := DeleteUser( store , doomed.ID ); err != nil { t.Fatal( err ) }
	if _ , err := GetUser( store , doomed.ID ); !errors.Is( err , db.ErrNotFound ) {
		t.Fatalf( "user still there: %v" , err )
	}
	owners := func( bucket string , newItem func() any , owner func( any ) uint64 ) ( found map[ uint64 ]int ) {
		found = map[ uint64 ]int{}
		store.ForEach( bucket , newItem , func( key []byte , item any ) bool {
			found[ owner( item ) ]++
			return true
		} )
		return
	}
	checks := map[ string ]map[ uint64 ]int{
		"sessions": owners( db.BucketSessions , func() any { return &Session{} } , func( item any ) uint64 { return item.( *Session ).UserID } ),
		"tokens":   owners( db.BucketLoginTokens , func() any { return &LoginToken{} } , func( item any ) uint64 { return item.( *LoginToken ).UserID } ),
		"keys":     owners( db.BucketAPIKeys , func() any { return &APIKey{} } , func( item any ) uint64 { return item.( *APIKey ).UserID } ),
	}
	for name , found := range checks {
		if found[ doomed.ID ] != 0 || found[ kept.ID ] != 1 {
			t.Fatalf( "%s after delete: %v" , name , found )
		}
	}
}

func TestNewUsersStartWithTheOwnersDefaults( t *testing.T ) {
	store := testStore( t )
	// Nothing saved yet: everything allowed, Master everywhere, extras on.
	fresh , err := CreateUser( store , "Ana" , RoleUser )
	if err != nil { t.Fatal( err ) }
	if !fresh.CanInvite || !fresh.CanHint || !fresh.CanUndo || !fresh.CanReshuffle {
		t.Fatalf( "permissions should start on: %+v" , fresh )
	}
	if fresh.HintLevel != "master" || fresh.BotLevel != "master" || fresh.AutoPlayLevel != "master" {
		t.Fatalf( "levels should start at master: %+v" , fresh )
	}
	if fresh.PlayerScores == nil || !*fresh.PlayerScores || fresh.LedGlow == nil || !*fresh.LedGlow {
		t.Fatalf( "table extras should start on: %+v" , fresh )
	}

	// The owner changes them: the next account gets the new choice and the
	// one already made keeps what it had.
	settings := AppSettings{
		NewUserNoInvite: true , NewUserNoHint: true , NewUserNoUndo: true , NewUserNoReshuffle: true ,
		NewUserHintLevel: "easy" , NewUserBotLevel: "hard" , NewUserAutoPlayLevel: "normal" ,
		NewUserNoPlayerScores: true , NewUserNoLedGlow: true ,
	}
	if err := SaveSettings( store , settings ); err != nil { t.Fatal( err ) }
	later , err := CreateUser( store , "Ben" , RoleUser )
	if err != nil { t.Fatal( err ) }
	if later.CanInvite || later.CanHint || later.CanUndo || later.CanReshuffle {
		t.Fatalf( "permissions should start off: %+v" , later )
	}
	if later.HintLevel != "easy" || later.BotLevel != "hard" || later.AutoPlayLevel != "normal" {
		t.Fatalf( "levels not copied: %+v" , later )
	}
	if *later.PlayerScores || *later.LedGlow {
		t.Fatalf( "table extras should start off: %+v" , later )
	}
	again , err := GetUser( store , fresh.ID )
	if err != nil { t.Fatal( err ) }
	if !again.CanHint || again.BotLevel != "master" {
		t.Fatalf( "an existing account changed: %+v" , again )
	}
}

func TestValidNewUserLevel( t *testing.T ) {
	for _ , level := range []string{ "" , "easy" , "master" } {
		if ValidNewUserLevel( level ) == false { t.Fatalf( "%q should be allowed" , level ) }
	}
	for _ , level := range []string{ "Master" , "godlike" , " easy" } {
		if ValidNewUserLevel( level ) { t.Fatalf( "%q should be refused" , level ) }
	}
}
