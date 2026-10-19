package models

import (
	errors "errors"
	time "time"

	db "rook/server/db"
)

// GlobalChatLine is one message in the chat everyone shares, across every
// table. The name and avatar are the sender's at the time they said it, the
// same way a table's chat keeps the name a seat had.
type GlobalChatLine struct {
	Seq    int       `json:"seq"`
	UserID uint64    `json:"user_id"`
	Name   string    `json:"name"`
	Icon   string    `json:"icon,omitempty"`
	Color  string    `json:"color,omitempty"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

// The recent lines are kept as one record, rewritten on every message. The
// tables package caps it at a hundred lines, so that stays small.
var globalChatKey = []byte( "global_chat" )

func LoadGlobalChat( store *db.Store ) ( lines []GlobalChatLine , err error ) {
	lines = []GlobalChatLine{}
	err = store.Get( db.BucketMeta , globalChatKey , &lines )
	if errors.Is( err , db.ErrNotFound ) { err = nil }
	return
}

func SaveGlobalChat( store *db.Store , lines []GlobalChatLine ) ( err error ) {
	err = store.Put( db.BucketMeta , globalChatKey , &lines )
	return
}
