package models

import (
	errors "errors"

	db "rook/server/db"
	encryption "rook/server/encryption"
)

// The superadmin is the account the server was set up with. Any admin may
// promote or demote anyone else, themselves included, so something has to
// stop the last way in from being taken away by accident: this one account
// can never be demoted, disabled or deleted.
//
// Its id is written down the first time anyone asks, rather than worked out
// each time. "The lowest-numbered admin" would move the moment an older
// plain account was promoted.

var superadminKey = []byte( "superadmin_id" )

// SuperAdminID returns the protected account's id, or 0 on a database with
// no active admin yet.
func SuperAdminID( store *db.Store ) ( user_id uint64 , err error ) {
	err = store.Get( db.BucketMeta , superadminKey , &user_id )
	if err == nil && user_id != 0 {
		if _ , get_err := GetUser( store , user_id ); get_err == nil { return }
	}
	if err != nil && errors.Is( err , db.ErrNotFound ) == false { return }
	err = nil

	// Not recorded yet, or the recorded account is gone (deleted from the
	// shell): pick the oldest active admin.
	user_id = 0
	err = store.ForEach( db.BucketUsers ,
		func() any { return &User{} } ,
		func( key []byte , item any ) bool {
			user := item.( *User )
			if user.IsAdmin() && user.Disabled() == false && ( user_id == 0 || user.ID < user_id ) {
				user_id = user.ID
			}
			return true
		} )
	if err != nil || user_id == 0 { return }
	err = store.Put( db.BucketMeta , superadminKey , user_id )
	return
}

// IsSuperAdmin answers for one account. A lookup failure answers yes, so a
// broken read refuses the change instead of allowing it.
func IsSuperAdmin( store *db.Store , user_id uint64 ) ( result bool ) {
	super_id , err := SuperAdminID( store )
	result = err != nil || ( super_id != 0 && super_id == user_id )
	return
}

func SetUserRole( store *db.Store , user_id uint64 , role string ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).Role = role
			return
		} )
	return
}
