package tables

import (
	fmt "fmt"

	game "rook/server/game"
)

// The game record is the log tab's view of a game: every hand with its
// auction, trump and tricks, built by replaying the move history. It is
// public -- anyone who may see the table may see it -- so it carries only
// what everyone saw happen: never a deck, a hand, or the buried cards (those
// arrive with the hand's result, when the bidder turns them over anyway).

type RecordBid struct {
	Seat   int `json:"seat"`
	Amount int `json:"amount"` // 0 is a pass
}

type RecordPlay struct {
	Seat int    `json:"seat"`
	Card string `json:"card"`
}

type RecordTrick struct {
	Number int          `json:"number"`
	Leader int          `json:"leader"`
	Plays  []RecordPlay `json:"plays"`
	Winner int          `json:"winner"` // -1 while the trick is still being played
	Points int          `json:"points"`
	Done   bool         `json:"done"`
}

type RecordResult struct {
	Made      bool   `json:"made"`
	Scored    [2]int `json:"scored"`
	Totals    [2]int `json:"totals"`
	Taken     [2]int `json:"taken"`
	TricksWon [2]int `json:"tricks_won"`
}

type RecordHand struct {
	Hand        int           `json:"hand"`
	FirstBidder int           `json:"first_bidder"`
	Bids        []RecordBid   `json:"bids"`
	Bidder      int           `json:"bidder"` // -1 while the auction is open
	Bid         int           `json:"bid"`
	Trump       string        `json:"trump,omitempty"`
	Tricks      []RecordTrick `json:"tricks"`
	Result      *RecordResult `json:"result,omitempty"`
	// Partial marks a hand known only from the score sheet: it was played
	// before this table kept a move history.
	Partial bool `json:"partial,omitempty"`
}

type GameRecord struct {
	Hands []RecordHand `json:"hands"`
}

// walk replays the log up to the cursor on a copy of Base, calling visit
// after each move with the state it produced and how many tricks the hand
// had finished before it. It never touches table.Game. Called with the table
// locked.
func ( table *Table ) walk( visit func( index int , action Action , state *game.State , completed_before int ) ) ( err error ) {
	state , err := cloneState( table.Base )
	if err != nil { return }
	for index , action := range table.Log[ :table.Cursor ] {
		completed_before := 0
		if state != nil && action.Kind != ActionStart { completed_before = len( state.Completed ) }
		state , _ , err = action.apply( state , table.Options )
		if err != nil {
			err = fmt.Errorf( "replaying move %d (%s): %w" , index+1 , action.Kind , err )
			return
		}
		visit( index , action , state , completed_before )
	}
	return
}

func recordTrick( number int , trick game.Trick , done bool ) ( result RecordTrick ) {
	result = RecordTrick{ Number: number , Leader: trick.Leader , Winner: -1 , Plays: []RecordPlay{} , Done: done }
	if done {
		result.Winner , result.Points = trick.Winner , trick.Points
	}
	for _ , play := range trick.Plays {
		result.Plays = append( result.Plays , RecordPlay{ Seat: play.Seat , Card: play.Card.ID() } )
	}
	if result.Leader < 0 && len( result.Plays ) > 0 { result.Leader = result.Plays[ 0 ].Seat }
	return
}

func newRecordHand( state *game.State ) ( hand RecordHand ) {
	hand = RecordHand{ Hand: state.HandNumber , FirstBidder: state.BidStarter , Bids: []RecordBid{} , Bidder: -1 , Tricks: []RecordTrick{} }
	return
}

// fromState fills in what a hand already shows: used for the hand a
// pre-history game was saved in the middle of.
func ( hand *RecordHand ) fromState( state *game.State ) {
	if state.Bidder >= 0 {
		hand.Bidder , hand.Bid = state.Bidder , state.Bid
	}
	if state.Trump != game.NoColor { hand.Trump = state.Trump.String() }
	for number , trick := range state.Completed {
		hand.Tricks = append( hand.Tricks , recordTrick( number+1 , trick , true ) )
	}
}

func ( hub *Hub ) Record( id string ) ( record GameRecord , err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	table.mu.Lock()
	defer table.mu.Unlock()
	record , err = table.record()
	return
}

// record builds the game record. Called with the table locked; the game
// history (archive.go) keeps a copy of it when a game ends.
func ( table *Table ) record() ( record GameRecord , err error ) {
	record.Hands = []RecordHand{}
	if table.Game == nil { return }
	hands := []RecordHand{}
	if table.Base != nil {
		start := newRecordHand( table.Base )
		start.fromState( table.Base )
		hands = append( hands , start )
	}
	current := func() *RecordHand { return &hands[ len( hands )-1 ] }

	err = table.walk( func( index int , action Action , state *game.State , completed_before int ) {
		switch action.Kind {
		case ActionStart:
			// A rematch is a new game; the record is this game's.
			hands = []RecordHand{ newRecordHand( state ) }
			return
		case ActionContinue:
			if len( hands ) == 0 || state.HandNumber != current().Hand {
				hands = append( hands , newRecordHand( state ) )
			}
			return
		case ActionReshuffle:
			// The same hand dealt again: what was bid on the old cards is gone.
			if len( hands ) > 0 && state.HandNumber == current().Hand {
				hands[ len( hands )-1 ] = newRecordHand( state )
			}
			return
		}
		if len( hands ) == 0 { return }
		hand := current()
		switch action.Kind {
		case ActionBid:
			hand.Bids = append( hand.Bids , RecordBid{ Seat: action.Seat , Amount: action.Amount } )
		case ActionPass:
			hand.Bids = append( hand.Bids , RecordBid{ Seat: action.Seat } )
		case ActionBury:
			hand.Trump = action.Trump
		case ActionPlay:
			if len( state.Completed ) > completed_before {
				hand.Tricks = append( hand.Tricks , recordTrick( len( state.Completed ) , state.Completed[ len( state.Completed )-1 ] , true ) )
			}
		}
		if state.Bidder >= 0 {
			hand.Bidder , hand.Bid = state.Bidder , state.Bid
		}
	} )
	if err != nil { return }

	// The trick on the table now, and each finished hand's result from the
	// score sheet. Hands the sheet knows about but the history does not (a
	// pre-history game) go first, as summaries.
	state := table.Game
	if len( hands ) > 0 && state.Phase == game.PhasePlaying && len( state.Trick.Plays ) > 0 {
		hand := current()
		hand.Tricks = append( hand.Tricks , recordTrick( len( state.Completed )+1 , state.Trick , false ) )
	}
	known := map[ int ]int{}
	for index , hand := range hands {
		known[ hand.Hand ] = index
	}
	earlier := []RecordHand{}
	for _ , result := range state.History {
		summary := &RecordResult{ Made: result.Made , Scored: result.Scored , Totals: result.Totals , Taken: result.Taken , TricksWon: result.TricksWon }
		if index , ok := known[ result.Hand ]; ok {
			hands[ index ].Result = summary
			continue
		}
		earlier = append( earlier , RecordHand{
			Hand: result.Hand , Bidder: result.Bidder , Bid: result.Bid , Trump: result.Trump.String() ,
			Bids: []RecordBid{} , Tricks: []RecordTrick{} , Result: summary , Partial: true ,
		} )
	}
	record.Hands = append( earlier , hands... )
	return
}
