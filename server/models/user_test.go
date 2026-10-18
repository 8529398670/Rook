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
