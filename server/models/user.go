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

// An account's kind. A person signs in; a service account (ProjectPortal's
// own, see portal.go) only ever has API keys. Rows written before kinds
// existed have none, and read as a person (User.KindOrPerson).
const (
	KindPerson  = "person"
	KindService = "service"
)

const (
	RoleAdmin  = "admin"
	RoleUser   = "user"
	// RoleViewer may read everything a user may, and change nothing: the
	// server answers 403 view_only to anything but a read (see
	// security.ViewOnly). Ranked viewer < user < admin.
	RoleViewer = "viewer"
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
	// ExpiresAt is when the account stops working, nil for never. Once it
	// passes the account counts as disabled everywhere (Disabled). Set by an
	// admin (SetUserExpiry), never on the owner. Rook has no housekeeping
	// job, so nothing flips DisabledAt later: Disabled is what refuses it.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
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
	// CanReshuffle lets a player deal the hand again, as often as they
	// like, until they have bid or passed in it. Off by default.
	CanReshuffle bool `json:"can_reshuffle,omitempty"`
	// Icon and Color are the avatar a player chose (see profile.go).
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
	// PinHash protects signing back in through a multi-use sign-up link.
	// Empty means no PIN: picking the name is enough. See signup_link.go.
	PinHash string `json:"pin_hash,omitempty"`
	// BotLevel is the level the Add bot picker starts on for this player
	// (game.Level's names). Empty means normal.
	BotLevel string `json:"bot_level,omitempty"`
	// AutoPlayLevel is the level auto play plays this player's cards at.
	// Empty means game.HelpLevel (Expert).
	AutoPlayLevel string `json:"auto_play_level,omitempty"`
	// HintLevel is the level the hint button suggests at for this player.
	// Empty means game.HelpLevel (Expert), as hints always were.
	HintLevel string `json:"hint_level,omitempty"`
	// PlayerScores and LedGlow are where the table's "points in the top
	// bar" and "outline the card that was led" switches start on a device
	// where the player has not flipped them yet (the switch itself is
	// remembered per device). Nil means off, as they always were.
	PlayerScores *bool `json:"player_scores,omitempty"`
	LedGlow      *bool `json:"led_glow,omitempty"`
	// WinGlow is the same for "outline the card that won the trick", except
	// that nil means on: the winner was always outlined before it had a switch.
	WinGlow *bool `json:"win_glow,omitempty"`
	// PartnerBotOutbids lets this player's bot partner keep raising them
	// after both opponents have passed. Off -- the zero value -- is the
	// default: the bot passes, since the hand is the team's already.
	PartnerBotOutbids bool `json:"partner_bot_outbids,omitempty"`
	// Kind is KindPerson or KindService; empty is a person.
	Kind string `json:"kind,omitempty"`
	// Themes is this person's own list on the Appearance screen; nil follows
	// the app's defaults. ThemeID is the one they use. See themes.go.
	// ThemesBuiltIns is which built-in themes that list has seen (themes.go).
	Themes         []Theme `json:"themes,omitempty"`
	ThemesBuiltIns int     `json:"themes_builtins,omitempty"`
	ThemeID        string  `json:"theme_id,omitempty"`
}

// MayInvite is the single answer to "may this account bring someone new in".
func ( user *User ) MayInvite() ( result bool ) {
	result = user.IsAdmin() || user.CanInvite
	return
}

func ( user *User ) Disabled() ( result bool ) {
	result = user.DisabledAt != nil || user.Expired()
	return
}

// Expired is true once ExpiresAt has passed.
func ( user *User ) Expired() ( result bool ) {
	result = user.ExpiresAt != nil && time.Now().UTC().After( *user.ExpiresAt )
	return
}

// KindOrPerson is the account's kind, a person when none was written.
func ( user *User ) KindOrPerson() ( kind string ) {
	kind = user.Kind
	if kind == "" { kind = KindPerson }
	return
}

// IsService is true for a service account: keys only, never signed into.
func ( user *User ) IsService() ( result bool ) {
	result = user.Kind == KindService
	return
}

func ( user *User ) IsAdmin() ( result bool ) {
	result = user.Role == RoleAdmin
	return
}

// RoleRank orders the roles, viewer < user < admin. An unknown role ranks
// lowest, below a viewer, so a bad row never grants anything.
func RoleRank( role string ) ( rank int ) {
	switch role {
	case RoleViewer:
		rank = 1
	case RoleUser:
		rank = 2
	case RoleAdmin:
		rank = 3
	}
	return
}

// LowerRole is the lower of two roles: what a key may do, given its own role
// and its account's.
func LowerRole( a string , b string ) ( result string ) {
	result = a
	if RoleRank( b ) < RoleRank( a ) { result = b }
	if RoleRank( result ) == 0 { result = RoleViewer }
	return
}

func ValidRole( role string ) ( result bool ) {
	result = role == RoleAdmin || role == RoleUser || role == RoleViewer
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
	// A new account starts with what the owner chose for new players.
	settings , err := LoadSettings( store )
	if err != nil {
		user = nil
		return
	}
	settings.ApplyNewUserDefaults( user )
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
		// Before the key rows go: voidWhatKeysMadeTx finds the keys by
		// account, and what they made in other accounts ends with them (S22).
		if tx_err = voidWhatKeysMadeTx( tx , user_id ); tx_err != nil { return }
		if tx_err = deleteAPIKeysForUser( tx , user_id ); tx_err != nil { return }
		tx_err = tx.Delete( db.BucketUsers , encryption.Uint64ToBytes( user_id ) )
		return
	} )
	return
}

// RenameConflictError is RenameUser's answer when the name was not what the
// caller expected: someone changed it since. Current is the name it has now.
type RenameConflictError struct {
	Current string
}

func ( conflict *RenameConflictError ) Error() ( result string ) {
	result = "the name was changed somewhere else in the meantime"
	return
}

