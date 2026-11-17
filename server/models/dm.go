package models

import (
	strconv "strconv"
	time "time"

	db "rook/server/db"
)

// DMLine is one direct message. The sender's name is the one they had when
// they said it, as in the global chat.
type DMLine struct {
	Seq    int       `json:"seq"`
	UserID uint64    `json:"user_id"`
	Name   string    `json:"name"`
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
	Edited bool      `json:"edited,omitempty"`
}

// DMConversation is everything two people have said to each other: the
// recent lines, how far each has read, and whether each has its tab open.
// Users is the pair, lower id first; Read and Open are keyed by user id.
type DMConversation struct {
	Users [2]uint64          `json:"users"`
	Seq   int                `json:"seq"`
	Lines []DMLine           `json:"lines"`
	Read  map[ uint64 ]int   `json:"read,omitempty"`
	Open  map[ uint64 ]bool  `json:"open,omitempty"`
}

// DMKey names the conversation between two people, whichever way round.
func DMKey( a uint64 , b uint64 ) ( key string ) {
	if b < a { a , b = b , a }
	key = strconv.FormatUint( a , 10 ) + ":" + strconv.FormatUint( b , 10 )
	return
}

// Other is the person in the conversation who is not user_id.
func ( conversation *DMConversation ) Other( user_id uint64 ) ( other uint64 ) {
	other = conversation.Users[ 0 ]
	if other == user_id { other = conversation.Users[ 1 ] }
	return
}

func ( conversation *DMConversation ) Has( user_id uint64 ) ( result bool ) {
	result = conversation.Users[ 0 ] == user_id || conversation.Users[ 1 ] == user_id
	return
}

func LoadDMs( store *db.Store ) ( conversations map[ string ]*DMConversation , err error ) {
	conversations = map[ string ]*DMConversation{}
	err = store.ForEach( db.BucketDMs ,
		func() any { return &DMConversation{} } ,
		func( key []byte , item any ) bool {
			conversation := item.( *DMConversation )
			if conversation.Read == nil { conversation.Read = map[ uint64 ]int{} }
			if conversation.Open == nil { conversation.Open = map[ uint64 ]bool{} }
			conversations[ string( key ) ] = conversation
			return true
		} )
	return
}

func SaveDM( store *db.Store , conversation *DMConversation ) ( err error ) {
	err = store.Put( db.BucketDMs , []byte( DMKey( conversation.Users[ 0 ] , conversation.Users[ 1 ] ) ) , conversation )
	return
}

func DeleteDM( store *db.Store , key string ) ( err error ) {
	err = store.Delete( db.BucketDMs , []byte( key ) )
	return
}
