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
	// shell): pick the oldest active admin. Never a service account
	// (ProjectPortal's, portal.go): it is an admin, and never the owner.
	user_id = 0
	err = store.ForEach( db.BucketUsers ,
		func() any { return &User{} } ,
		func( key []byte , item any ) bool {
			user := item.( *User )
			if user.IsAdmin() && user.Disabled() == false && user.IsService() == false && ( user_id == 0 || user.ID < user_id ) {
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

// SetUserRole changes an account's role. A service account is an admin,
// always (portal.go): changing it is ErrServiceAccount.
func SetUserRole( store *db.Store , user_id uint64 , role string ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			user := item.( *User )
			if user.IsService() && user.Role != role {
				mutate_err = ErrServiceAccount
				return
			}
			user.Role = role
			return
		} )
	return
}

// superAdminIDTx is SuperAdminID read inside the caller's transaction, for
// the portal contract v2 checks (expiry, key minting). It only reads what
// SuperAdminID recorded, which every owner-only route has done by the time it
// runs (RequireOwner asks IsSuperAdmin first). Zero means none recorded yet.
func superAdminIDTx( tx *db.Tx ) ( user_id uint64 , err error ) {
	err = tx.Get( db.BucketMeta , superadminKey , &user_id )
	if errors.Is( err , db.ErrNotFound ) { user_id , err = 0 , nil }
	return
}

// refuseSuperAdminTx is ErrOwnerProtected for the superadmin's account.
func refuseSuperAdminTx( tx *db.Tx , user_id uint64 ) ( err error ) {
	super_id , err := superAdminIDTx( tx )
	if err != nil { return }
	if super_id != 0 && super_id == user_id { err = ErrOwnerProtected }
	return
}

// maySuperActOnTx is mayActOnTx with Rook's superadmin as the owner: nobody
// acts on the superadmin's account, and only the superadmin acts on another
// admin's. actor_id zero is the shell.
func maySuperActOnTx( tx *db.Tx , actor_id uint64 , target *User ) ( err error ) {
	if actor_id == 0 || actor_id == target.ID { return }
	super_id , err := superAdminIDTx( tx )
	if err != nil { return }
	if super_id != 0 && target.ID == super_id {
		err = ErrOwnerProtected
		return
	}
	if target.IsAdmin() && actor_id != super_id {
		err = ErrOwnerRequired
	}
	return
}

// MaySuperActOn is maySuperActOnTx on its own read, for a route that must
// refuse before it does anything else (routes.RequireAccountAdmin). A
// missing account is db.ErrNotFound.
func MaySuperActOn( store *db.Store , actor_id uint64 , user_id uint64 ) ( err error ) {
	if actor_id == 0 || actor_id == user_id { return }
	err = store.Write( func( tx *db.Tx ) ( tx_err error ) {
		target := &User{}
		if tx_err = tx.Get( db.BucketUsers , encryption.Uint64ToBytes( user_id ) , target ); tx_err != nil { return }
		tx_err = maySuperActOnTx( tx , actor_id , target )
		return
	} )
	return
}

