// Package models holds one file per stored record type. There is no ORM and
// no query builder -- each file is a small set of plain functions over the db
// store, which is enough for a bolt key/value file and keeps the storage
// layer readable end to end.
package models

import (
	strings "strings"
	time "time"

	db "rook/server/db"
	encryption "rook/server/encryption"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User has no password field, and adding one would undo the entire design
// here: identity is proven only by redeeming a login link or presenting a
// session cookie. See references/architecture.md before changing that.
type User struct {
	ID          uint64     `json:"id"`
	DisplayName string     `json:"display_name"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
	// CanInvite lets a non-admin mint login links for new players from a
	// table's empty seat. Off by default: deciding who gets in is an admin's
	// call, and this is an admin handing a specific person part of it.
	CanInvite bool `json:"can_invite,omitempty"`
	// CanHint shows the hint button to this player. Off by default; an
	// admin decides who gets help.
	CanHint bool `json:"can_hint,omitempty"`
	// CanUndo lets a player step back and forth through the moves of a game
	// they are sitting in -- undo, redo, reset to a move, resume -- as an
	// admin can at any table. Off by default.
	CanUndo bool `json:"can_undo,omitempty"`
	// Icon and Color are the avatar a player chose (see profile.go).
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
	// PinHash protects signing back in through a multi-use sign-up link.
	// Empty means no PIN: picking the name is enough. See signup_link.go.
	PinHash string `json:"pin_hash,omitempty"`
}

// MayInvite is the single answer to "may this account bring someone new in".
func ( user *User ) MayInvite() ( result bool ) {
	result = user.IsAdmin() || user.CanInvite
	return
}

func ( user *User ) Disabled() ( result bool ) {
	result = user.DisabledAt != nil
	return
}

func ( user *User ) IsAdmin() ( result bool ) {
	result = user.Role == RoleAdmin
	return
}

func ValidRole( role string ) ( result bool ) {
	result = role == RoleAdmin || role == RoleUser
	return
}

// ValidDisplayName bounds the one piece of free text a user controls. The cap
// is about keeping the database and the UI sane; escaping for display is the
// frontend's job (see static/js/dom.js, which never uses innerHTML for
// user-supplied values).
func ValidDisplayName( display_name string ) ( result bool ) {
	trimmed := strings.TrimSpace( display_name )
	result = len( trimmed ) >= 1 && len( trimmed ) <= 80
	return
}

func CreateUser( store *db.Store , display_name string , role string ) ( user *User , err error ) {
	next_id , err := store.NextSequence( db.BucketUsers )
	if err != nil { return }
	user = &User{
		ID:          next_id,
		DisplayName: strings.TrimSpace( display_name ),
		Role:        role,
		CreatedAt:   time.Now().UTC(),
	}
	err = store.Put( db.BucketUsers , encryption.Uint64ToBytes( user.ID ) , user )
	if err != nil { user = nil }
	return
}

func GetUser( store *db.Store , user_id uint64 ) ( user *User , err error ) {
	user = &User{}
	err = store.Get( db.BucketUsers , encryption.Uint64ToBytes( user_id ) , user )
	if err != nil { user = nil }
	return
}

func ListUsers( store *db.Store ) ( users []*User , err error ) {
	users = []*User{}
	err = store.ForEach( db.BucketUsers ,
		func() any { return &User{} } ,
		func( key []byte , item any ) bool {
			users = append( users , item.( *User ) )
			return true
		} )
	return
}

func SaveUser( store *db.Store , user *User ) ( err error ) {
	err = store.Put( db.BucketUsers , encryption.Uint64ToBytes( user.ID ) , user )
	return
}

// RenameUser and SetUserDisabled both edit a record that already exists, so
// the read and the write land in one transaction rather than GetUser followed
// by SaveUser. That is what stops two concurrent admin requests from each
// reading the old row and the slower one silently discarding the other's
// change.
func RenameUser( store *db.Store , user_id uint64 , display_name string ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).DisplayName = strings.TrimSpace( display_name )
			return
		} )
	return
}

// SetUserDisabled disables or re-enables an account. Disabling also deletes
// every session the account holds and every login link it has not used yet,
// in the same transaction as the change itself: a session or a link that
// survived would come back to life the moment the account was re-enabled.
// API keys are deliberately left alone -- see LoadAPIKey -- and use
// RevokeAllCredentials when they should die too.
func SetUserDisabled( store *db.Store , user_id uint64 , disabled bool ) ( err error ) {
	err = store.Write( func( tx *db.Tx ) ( tx_err error ) {
		key := encryption.Uint64ToBytes( user_id )
		user := &User{}
		if tx_err = tx.Get( db.BucketUsers , key , user ); tx_err != nil { return }
		if disabled {
			now := time.Now().UTC()
			user.DisabledAt = &now
		} else {
			user.DisabledAt = nil
		}
		if tx_err = tx.Put( db.BucketUsers , key , user ); tx_err != nil { return }
		if disabled == false { return }
		if _ , tx_err = destroySessionsForUser( tx , user_id ); tx_err != nil { return }
		_ , tx_err = deletePendingLoginTokens( tx , user_id )
		return
	} )
	return
}

// AnyAdminExists decides whether the server prints a first-run login link on
// boot. Once one admin exists, that bootstrap path stays closed.
func AnyAdminExists( store *db.Store ) ( result bool , err error ) {
	err = store.ForEach( db.BucketUsers ,
		func() any { return &User{} } ,
		func( key []byte , item any ) bool {
			user := item.( *User )
			if user.IsAdmin() && user.Disabled() == false {
				result = true
				return false
			}
			return true
		} )
	return
}

func SetUserCanInvite( store *db.Store , user_id uint64 , can_invite bool ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).CanInvite = can_invite
			return
		} )
	return
}

// CountActiveAdmins is the guard rail against deleting the last way in: with
// no enabled admin left, nobody can mint a login link again.
func CountActiveAdmins( store *db.Store ) ( count int , err error ) {
	err = store.ForEach( db.BucketUsers ,
		func() any { return &User{} } ,
		func( key []byte , item any ) bool {
			user := item.( *User )
			if user.IsAdmin() && user.Disabled() == false { count++ }
			return true
		} )
	return
}

// DeleteUser removes an account for good, along with everything that could
// still sign in as it: sessions, login links and API keys. All of it is one
// transaction, so a request racing the delete either signs in first (and
// that session goes with the rest) or finds no account at all.
func DeleteUser( store *db.Store , user_id uint64 ) ( err error ) {
	err = store.Write( func( tx *db.Tx ) ( tx_err error ) {
		if _ , tx_err = destroySessionsForUser( tx , user_id ); tx_err != nil { return }
		if tx_err = deleteLoginTokensForUser( tx , user_id ); tx_err != nil { return }
		if tx_err = deleteAPIKeysForUser( tx , user_id ); tx_err != nil { return }
		tx_err = tx.Delete( db.BucketUsers , encryption.Uint64ToBytes( user_id ) )
		return
	} )
	return
}
