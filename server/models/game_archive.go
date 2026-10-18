package models

import (
	json "encoding/json"

	db "rook/server/db"
)

// Archived games are the game history: one row per game that finished or was
// abandoned, kept after its table is gone. Like a table, the row is an opaque
// JSON document owned by server/tables (see archive.go there). Keys sort by
// when the game ended, so walking the bucket backwards is newest first.
type StoredGame struct {
	ID       string          `json:"id"`
	Document json.RawMessage `json:"document"`
}

func SaveGame( store *db.Store , id string , document json.RawMessage ) ( err error ) {
	err = store.Put( db.BucketGames , []byte( id ) , &StoredGame{ ID: id , Document: document } )
	return
}

func GetGame( store *db.Store , id string ) ( game *StoredGame , err error ) {
	game = &StoredGame{}
	err = store.Get( db.BucketGames , []byte( id ) , game )
	if err != nil { game = nil }
	return
}

func DeleteGame( store *db.Store , id string ) ( err error ) {
	err = store.Delete( db.BucketGames , []byte( id ) )
	return
}

// ListGames returns up to limit games, newest first, skipping the first
// offset of them.
func ListGames( store *db.Store , offset int , limit int ) ( games []*StoredGame , err error ) {
	games = []*StoredGame{}
	skipped := 0
	err = store.ForEachReverse( db.BucketGames ,
		func() any { return &StoredGame{} } ,
		func( key []byte , item any ) bool {
			if skipped < offset {
				skipped++
				return true
			}
			games = append( games , item.( *StoredGame ) )
			return len( games ) < limit
		} )
	return
}
