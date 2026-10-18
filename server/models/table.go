package models

import (
	json "encoding/json"

	db "rook/server/db"
)

// Tables are stored as an opaque JSON document owned by server/tables. This
// file only knows how to put one in the bucket and get it back out; keeping
// the shape over there means the model and the hub cannot drift apart, and
// store.Put still encrypts it at rest like everything else -- which matters
// here, since a saved table holds every player's hand.
type StoredTable struct {
	ID       string          `json:"id"`
	Document json.RawMessage `json:"document"`
}

func SaveTable( store *db.Store , id string , document json.RawMessage ) ( err error ) {
	err = store.Put( db.BucketTables , []byte( id ) , &StoredTable{ ID: id , Document: document } )
	return
}

func DeleteTable( store *db.Store , id string ) ( err error ) {
	err = store.Delete( db.BucketTables , []byte( id ) )
	return
}

func ListTables( store *db.Store ) ( tables []*StoredTable , err error ) {
	tables = []*StoredTable{}
	err = store.ForEach( db.BucketTables ,
		func() any { return &StoredTable{} } ,
		func( key []byte , item any ) bool {
			tables = append( tables , item.( *StoredTable ) )
			return true
		} )
	return
}
