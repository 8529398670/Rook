package models

import (
	testing "testing"
)

// The superadmin is pinned the first time it is asked for: promoting an
// older plain account afterwards must not move it.
func TestSuperAdminStaysPut( t *testing.T ) {
	store := testStore( t )
	if id , err := SuperAdminID( store ); err != nil || id != 0 {
		t.Fatalf( "with nobody: %d %v" , id , err )
	}
	plain , _ := CreateUser( store , "Ana" , RoleUser )
	first , _ := CreateUser( store , "Ben" , RoleAdmin )
	if id , _ := SuperAdminID( store ); id != first.ID {
		t.Fatalf( "superadmin %d, want %d" , id , first.ID )
	}
	if err := SetUserRole( store , plain.ID , RoleAdmin ); err != nil { t.Fatal( err ) }
	if id , _ := SuperAdminID( store ); id != first.ID {
		t.Fatalf( "superadmin moved to %d after a promotion" , id )
	}
	if IsSuperAdmin( store , first.ID ) == false || IsSuperAdmin( store , plain.ID ) {
		t.Fatalf( "IsSuperAdmin disagrees" )
	}
	if user , _ := GetUser( store , plain.ID ); user.IsAdmin() == false {
		t.Fatalf( "promotion not saved" )
	}
}
