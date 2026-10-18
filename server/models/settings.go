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
