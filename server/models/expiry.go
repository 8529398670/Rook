package models

import (
	errors "errors"
	time "time"

	db "rook/server/db"
	encryption "rook/server/encryption"
)

// Account expiry: an account that works until a set time, for a guest, a
// contractor, a trial. Portal contract v2 (capability account_expiry).
//
// Once ExpiresAt passes the account counts as disabled (User.Disabled), so
// sign-in, its sessions and its keys stop at that moment, read on every
// request. Rook has no housekeeping job, so nothing disables the account for
// real later: moving the expiry later, or clearing it, lets it back in.

// ErrExpiryPast refuses an expiry that is not in the future: expiring an
// account now is disabling it, which has its own route.
var ErrExpiryPast = errors.New( "expires_at must be in the future" )

// CreateUserExpiring is CreateUser for an account that stops working at
// expires_at (nil for never). Rook's CreateUser is not one transaction with
// anything else, so the expiry is set straight after; if that fails, the new
// account is removed again rather than left without it.
func CreateUserExpiring( store *db.Store , display_name string , role string , expires_at *time.Time ) ( user *User , err error ) {
	if expires_at != nil && expires_at.After( time.Now() ) == false {
		err = ErrExpiryPast
		return
	}
	if user , err = CreateUser( store , display_name , role ); err != nil || expires_at == nil { return }
	created := user
	if user , err = SetUserExpiry( store , created.ID , 0 , expires_at ); err != nil {
		err = errors.Join( err , DeleteUser( store , created.ID ) )
		user = nil
	}
	return
}

// SetUserExpiry sets when an account stops working, nil for never. It must
// be in the future (ErrExpiryPast). The owner never expires
// (ErrOwnerProtected), and only the owner sets an admin's. Rook's owner is
// its superadmin (superadmin.go), so the checks read that.
func SetUserExpiry( store *db.Store , user_id uint64 , actor_id uint64 , expires_at *time.Time ) ( user *User , err error ) {
	if expires_at != nil && expires_at.After( time.Now() ) == false {
		err = ErrExpiryPast
		return
	}
	err = store.Write( func( tx *db.Tx ) ( tx_err error ) {
		key := encryption.Uint64ToBytes( user_id )
		found := &User{}
		if tx_err = tx.Get( db.BucketUsers , key , found ); tx_err != nil { return }
		if tx_err = refuseSuperAdminTx( tx , user_id ); tx_err != nil { return }
		if tx_err = maySuperActOnTx( tx , actor_id , found ); tx_err != nil { return }
		found.ExpiresAt = nil
		if expires_at != nil {
			at := expires_at.UTC()
			found.ExpiresAt = &at
		}
		if tx_err = tx.Put( db.BucketUsers , key , found ); tx_err != nil { return }
		user = found
		return
	} )
	if err != nil { user = nil }
	return
}
