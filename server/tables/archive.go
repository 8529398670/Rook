package tables

import (
	json "encoding/json"
	fmt "fmt"
	log "log"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

// The game history: a copy of every game that finished, or that was given up
// on before it did, kept after its table is gone. The live table is purged
// a few days after it goes quiet; the archive row stays.
//
// A game is known by where it starts in the table's move log, so a rematch
// at the same table is a separate game. Archives maps each game (by that
// start) to its row, which is what keeps one game to one row: an admin undo
// that reopens a finished game takes its row back out, and finishing it again
// writes a fresh one.

// ArchiveRef is a table's note of one of its games in the history.
type ArchiveRef struct {
	ID       string `json:"id"`
	Finished bool   `json:"finished,omitempty"`
}

const (
	GameFinished  = "finished"
	GameAbandoned = "abandoned"
)

type ArchivedSeat struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Bot   int    `json:"bot,omitempty"`
	Level string `json:"level,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

// GameSummary is one line of the history list.
type GameSummary struct {
	ID          string          `json:"id"`
	TableName   string          `json:"table_name"`
	Seats       [4]ArchivedSeat `json:"seats"`
	Status      string          `json:"status"`
	Winner      int             `json:"winner"` // the winning team, or -1
	Totals      [2]int          `json:"totals"`
	HandsPlayed int             `json:"hands_played"`
	EndedAt     time.Time       `json:"ended_at"`
}

type ArchivedGame struct {
	GameSummary
	Record GameRecord `json:"record"`
}

// gameStart is where the current game begins in the log: the last start at
// or before the cursor, or -1 for a game saved before history was kept.
func ( table *Table ) gameStart() ( index int ) {
	index = -1
	for position := table.Cursor - 1; position >= 0; position-- {
		if table.Log[ position ].Kind == ActionStart {
			index = position
			return
		}
	}
	return
}

func gameKey( start int ) ( key string ) {
	key = fmt.Sprint( start )
	return
}

// syncArchive keeps the history in step with the game after a change: a game
// that just ended is written, and one an undo reopened is taken back out.
// Called with the table locked, from mutate.
func ( hub *Hub ) syncArchive( table *Table ) {
	if table.Game == nil { return }
	key := gameKey( table.gameStart() )
	row , archived := table.Archives[ key ]
	over := table.Game.Phase == game.PhaseGameOver
	switch {
	case over && archived == false:
		hub.archive( table , GameFinished )
	case over == false && archived && row.Finished:
		// Only a finished game can be reopened. An abandoned one is gone
		// for good, even if a move that was already waiting lands on the
		// table after it was deleted.
		if err := models.DeleteGame( hub.store , row.ID ); err != nil {
			log.Printf( "[tables] could not remove archived game %s: %v" , row.ID , err )
		}
		delete( table.Archives , key )
	}
}

// archiveAbandoned keeps a game that is about to be thrown away -- its table
// deleted, purged, emptied, or restarted -- unless it is already in the
// history. A table that never dealt has no game to keep. Called with the
// table locked.
func ( hub *Hub ) archiveAbandoned( table *Table ) {
	if table.Game == nil { return }
	if _ , archived := table.Archives[ gameKey( table.gameStart() ) ]; archived { return }
	status := GameAbandoned
	if table.Game.Phase == game.PhaseGameOver { status = GameFinished }
	hub.archive( table , status )
}

// archive writes the current game to the history. The row's id starts with
// the time, so the bucket sorts oldest to newest.
func ( hub *Hub ) archive( table *Table , status string ) {
	start := table.gameStart()
	now := time.Now().UTC()
	entry := ArchivedGame{ GameSummary: GameSummary{
		ID:          fmt.Sprintf( "%016x-%s-%d" , now.UnixNano() , table.ID , start+1 ),
		TableName:   table.Name,
		Status:      status,
		Winner:      -1,
		Totals:      table.Game.Scores,
		HandsPlayed: len( table.Game.History ),
		EndedAt:     now,
	} }
	if status == GameFinished { entry.Winner = table.Game.Winner }
	for index , seat := range table.Seats {
		entry.Seats[ index ] = ArchivedSeat{ Kind: seat.Kind , Name: seat.shownName() , Bot: seat.Bot , Level: string( seat.Level ) , Icon: seat.Icon , Color: seat.Color }
	}
	record , record_err := table.record()
	if record_err != nil {
		// Keep the scores even if the hand-by-hand log cannot be rebuilt.
		log.Printf( "[tables] could not build the record of table %s for the history: %v" , table.ID , record_err )
		record = GameRecord{ Hands: []RecordHand{} }
	}
	entry.Record = record

	document , err := json.Marshal( entry )
	if err == nil { err = models.SaveGame( hub.store , entry.ID , document ) }
	if err != nil {
		log.Printf( "[tables] could not archive a game from table %s: %v" , table.ID , err )
		return
	}
	if table.Archives == nil { table.Archives = map[ string ]ArchiveRef{} }
	table.Archives[ gameKey( start ) ] = ArchiveRef{ ID: entry.ID , Finished: status == GameFinished }
}

// Games lists the history, newest first.
func ( hub *Hub ) Games( offset int , limit int ) ( games []GameSummary , err error ) {
	games = []GameSummary{}
	stored , err := models.ListGames( hub.store , offset , limit )
	if err != nil { return }
	for _ , row := range stored {
		var entry GameSummary
		if decode_err := json.Unmarshal( row.Document , &entry ); decode_err != nil {
			log.Printf( "[tables] skipping unreadable archived game %s: %v" , row.ID , decode_err )
			continue
		}
		games = append( games , entry )
	}
	return
}

// Game is one archived game in full.
func ( hub *Hub ) Game( id string ) ( entry ArchivedGame , err error ) {
	row , err := models.GetGame( hub.store , id )
	if err != nil {
		err = ErrNoSuchGame
		return
	}
	err = json.Unmarshal( row.Document , &entry )
	return
}

// DeleteGame takes one game out of the history for good. A table that is
// still open keeps its note that the game was archived, so the game is not
// written back when it next changes.
func ( hub *Hub ) DeleteGame( id string ) ( err error ) {
	if _ , get_err := models.GetGame( hub.store , id ); get_err != nil {
		err = ErrNoSuchGame
		return
	}
	err = models.DeleteGame( hub.store , id )
	return
}
