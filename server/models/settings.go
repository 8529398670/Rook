package models

import (
	errors "errors"

	db "rook/server/db"
)

// AppSettings are the few switches an admin flips from the Settings screen
// rather than from config.yaml: they are about how the game is run day to
// day, not how the server is deployed.
type AppSettings struct {
	// AllowJoinSignup lets someone who is not signed in create an account
	// from a table's join link, just by typing a name. Off by default: it is
	// self-service signup for anyone the link reaches.
	AllowJoinSignup bool `json:"allow_join_signup"`
	// ObserversSeeAll lets everyone watching any table see every hand.
	ObserversSeeAll bool `json:"observers_see_all"`
	// TableChatOff and GlobalChatOff switch off the chat at every table and
	// the chat everyone shares. Named for "off" so that a database saved
	// before they existed reads as both on.
	TableChatOff  bool `json:"table_chat_off"`
	GlobalChatOff bool `json:"global_chat_off"`
	// TrickPauseMS is how long a finished trick stays on the table before
	// it is swept to the winner, at every table that has not chosen its
	// own. 0 means DefaultTrickPauseMS, so a database saved before this
	// existed keeps the pause it always had.
	TrickPauseMS int `json:"trick_pause_ms,omitempty"`
	// RookAnytime and RookFirstTrickOff are the bird's house rules, copied
	// into each table when it is created (see game.Options), so a change
	// never alters a game already under way. RookFirstTrickOff is named for
	// "off" so a database saved before it existed keeps the bird allowed on
	// the first trick, as it always was.
	RookAnytime       bool `json:"rook_anytime"`
	RookFirstTrickOff bool `json:"rook_first_trick_off"`
}

// DefaultTrickPauseMS is the pause a finished trick has always had.
const DefaultTrickPauseMS = 1100

// TrickPauses are the pauses on offer, in milliseconds: a short list rather
// than any number, so a typo cannot freeze every table for a minute.
var TrickPauses = []int{ 500 , 800 , 1100 , 1500 , 2000 , 2500 , 3000 , 4000 , 5000 }

// ValidTrickPause accepts 0 ("the default") or one of TrickPauses.
func ValidTrickPause( ms int ) ( ok bool ) {
	if ms == 0 {
		ok = true
		return
	}
	for _ , choice := range TrickPauses {
		if choice == ms {
			ok = true
			return
		}
	}
	return
}

// TrickPause is the app-wide pause, with 0 read as the default.
func ( settings AppSettings ) TrickPause() ( ms int ) {
	ms = settings.TrickPauseMS
	if ms == 0 { ms = DefaultTrickPauseMS }
	return
}

var settingsKey = []byte( "app_settings" )

func LoadSettings( store *db.Store ) ( settings AppSettings , err error ) {
	err = store.Get( db.BucketMeta , settingsKey , &settings )
	if errors.Is( err , db.ErrNotFound ) { err = nil }
	return
}

func SaveSettings( store *db.Store , settings AppSettings ) ( err error ) {
	err = store.Put( db.BucketMeta , settingsKey , &settings )
	return
}
