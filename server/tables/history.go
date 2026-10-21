package tables

import (
	json "encoding/json"
	fmt "fmt"

	game "rook/server/game"
)

// Every move in a game is recorded as an Action, deck and all. Because the
// engine is deterministic -- the same deck and the same moves always produce
// the same position -- the log is the game: the current State is only the
// log replayed up to Cursor.
//
// That is what admin undo, redo, and "reset to here" are. Moving the cursor
// replays the log to a different point; making a move while the cursor is
// short of the end throws away the moves after it, like any undo history.
// Whenever an admin moves the cursor the table is put on hold, so nobody --
// bots included -- plays over the top of them until they press Resume.

const (
	ActionStart    = "start"
	ActionBid      = "bid"
	ActionPass     = "pass"
	ActionBury     = "bury"
	ActionPlay     = "play"
	ActionContinue = "continue"
	// ActionReshuffle deals the hand again before its player has bid.
	ActionReshuffle = "reshuffle"
)

type Action struct {
	Kind   string      `json:"kind"`
	Seat   int         `json:"seat"`
	Hand   int         `json:"hand"`
	Amount int         `json:"amount,omitempty"`
	Card   string      `json:"card,omitempty"`
	Cards  []string    `json:"cards,omitempty"`
	Trump  string      `json:"trump,omitempty"`
	// First is the opening bidder for a start.
	First int `json:"first,omitempty"`
	// Deck is recorded on every action that dealt cards, so a replay deals
	// exactly the same hands.
	Deck []game.Card `json:"deck,omitempty"`
	Bot  bool        `json:"bot,omitempty"`
}

// apply makes the move on state -- or, for a start, makes a new state -- and
// reports whether it dealt a fresh hand.
func ( action Action ) apply( state *game.State , options game.Options ) ( next *game.State , dealt bool , err error ) {
	if action.Kind == ActionStart {
		next , err = game.NewGameWithDeck( options , action.First , action.Deck )
		dealt = err == nil
		return
	}
	if state == nil {
		err = ErrNoGame
		return
	}
	next = state
	switch action.Kind {
	case ActionBid:
		err = state.PlaceBid( action.Seat , action.Amount )
	case ActionPass:
		err = state.Pass( action.Seat )
	case ActionBury:
		cards := make( []game.Card , 0 , len( action.Cards ) )
		for _ , id := range action.Cards {
			card , parse_err := game.ParseCard( id )
			if parse_err != nil {
				err = parse_err
				return
			}
			cards = append( cards , card )
		}
		trump , ok := game.ParseColor( action.Trump )
		if ok == false {
			err = game.ErrBadTrump
			return
		}
		err = state.Bury( action.Seat , cards , trump )
	case ActionPlay:
		card , parse_err := game.ParseCard( action.Card )
		if parse_err != nil {
			err = parse_err
			return
		}
		err = state.PlayCard( action.Seat , card )
	case ActionContinue:
		dealt , err = state.Continue( action.Seat , action.Deck )
	case ActionReshuffle:
		err = state.Reshuffle( action.Seat , action.Deck )
		dealt = err == nil
	default:
		err = ErrBadAction
	}
	return
}

// perform records a move and applies it. The engine checks every move before
// changing anything, so a refused move leaves both the state and the log as
// they were. Called with the table locked.
func ( table *Table ) perform( action Action ) ( err error ) {
	if table.Held {
		err = ErrHeld
		return
	}
	if action.Kind == ActionStart || action.Kind == ActionContinue || action.Kind == ActionReshuffle {
		action.Deck = game.ShuffledDeck( nil )
	}
	if table.Game != nil && action.Kind != ActionStart {
		action.Hand = table.Game.HandNumber
	} else {
		action.Hand = 1
	}

	next , dealt , err := action.apply( table.Game , table.Options )
	if err != nil { return }
	if dealt == false {
		// Only a deal needs its deck kept; the other "ready" clicks of a
		// hand summary carry nothing worth storing.
		action.Deck = nil
	}
	table.Game = next
	if dealt {
		table.AutoPlay = nil
		table.AutoPlayLevel = nil
	}
	table.Log = append( table.Log[ :table.Cursor ] , action )
	table.Cursor = len( table.Log )
	return
}

func cloneState( state *game.State ) ( copied *game.State , err error ) {
	if state == nil { return }
	raw , err := json.Marshal( state )
	if err != nil { return }
	copied = &game.State{}
	err = json.Unmarshal( raw , copied )
	return
}

// rebuild replays the log up to cursor. Base, when set, is where a game
// saved before this history existed starts from. Called with the table
// locked.
func ( table *Table ) rebuild( cursor int ) ( err error ) {
	if cursor < 0 || cursor > len( table.Log ) {
		err = ErrBadCursor
		return
	}
	state , err := cloneState( table.Base )
	if err != nil { return }
	for index , action := range table.Log[ :cursor ] {
		state , _ , err = action.apply( state , table.Options )
		if err != nil {
			err = fmt.Errorf( "replaying move %d (%s): %w" , index+1 , action.Kind , err )
			return
		}
	}
	table.Game = state
	table.Cursor = cursor
	return
}

// Undo, Redo, ResetTo and Resume are admin-only; the route checks the
// caller's effective role before calling them.

func ( hub *Hub ) Undo( id string ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if table.Cursor == 0 {
			fn_err = ErrNothingToUndo
			return
		}
		if fn_err = table.rebuild( table.Cursor - 1 ); fn_err == nil {
			table.Held = true
		}
		return
	} )
	return
}

func ( hub *Hub ) Redo( id string ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if table.Cursor >= len( table.Log ) {
			fn_err = ErrNothingToRedo
			return
		}
		if fn_err = table.rebuild( table.Cursor + 1 ); fn_err == nil {
			table.Held = true
		}
		return
	} )
	return
}

// ResetTo puts the game back to how it stood after the given number of
// moves. The moves after it stay in the log -- redo still reaches them --
// until somebody plays a different move from here.
func ( hub *Hub ) ResetTo( id string , cursor int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if fn_err = table.rebuild( cursor ); fn_err == nil {
			table.Held = true
		}
		return
	} )
	return
}

func ( hub *Hub ) Resume( id string ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if table.Held == false {
			fn_err = ErrNotHeld
			return
		}
		table.Held = false
		return
	} )
	return
}

// HistoryView is the admin's list of every move, with where the game stands.
// Entry n is "the position after n moves", so resetting to an entry's Index
// shows the table just after that move. Decks are left out: they are the
// whole deal in advance.
type HistoryView struct {
	Cursor  int            `json:"cursor"`
	Base    bool           `json:"base"`
	Entries []HistoryEntry `json:"entries"`
}

type HistoryEntry struct {
	Index  int      `json:"index"`
	Kind   string   `json:"kind"`
	Seat   int      `json:"seat"`
	Hand   int      `json:"hand"`
	Amount int      `json:"amount,omitempty"`
	Card   string   `json:"card,omitempty"`
	Cards  []string `json:"cards,omitempty"`
	Trump  string   `json:"trump,omitempty"`
	First  int      `json:"first"`
	Bot    bool     `json:"bot,omitempty"`
	// For a play: which trick of the hand it belongs to, and -- on the card
	// that finished the trick -- who took it and for how many points.
	// Winner is -1 everywhere else.
	Trick  int `json:"trick,omitempty"`
	Winner int `json:"winner"`
	Points int `json:"points,omitempty"`
}

func ( hub *Hub ) History( id string ) ( history HistoryView , err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	table.mu.Lock()
	defer table.mu.Unlock()
	history = HistoryView{ Cursor: table.Cursor , Base: table.Base != nil , Entries: []HistoryEntry{} }
	for index , action := range table.Log {
		history.Entries = append( history.Entries , HistoryEntry{
			Index: index + 1 , Kind: action.Kind , Seat: action.Seat , Hand: action.Hand ,
			Amount: action.Amount , Card: action.Card , Cards: action.Cards , Trump: action.Trump ,
			First: action.First , Bot: action.Bot , Winner: -1 ,
		} )
	}
	table.annotateTricks( history.Entries )
	return
}

// annotateTricks groups the plays into tricks for the history sheet. The
// moves after the cursor (what redo would reach) are replayed too, on a
// scratch copy, so the whole list is grouped. A replay that fails just
// leaves the plays ungrouped; the list itself is still right.
func ( table *Table ) annotateTricks( entries []HistoryEntry ) {
	scratch := &Table{ Options: table.Options , Base: table.Base , Log: table.Log , Cursor: len( table.Log ) }
	walk_err := scratch.walk( func( index int , action Action , state *game.State , completed_before int ) {
		if action.Kind != ActionPlay { return }
		entry := &entries[ index ]
		entry.Trick = completed_before + 1
		if len( state.Completed ) > completed_before {
			finished := state.Completed[ len( state.Completed )-1 ]
			entry.Winner , entry.Points = finished.Winner , finished.Points
		}
	} )
	if walk_err != nil {
		for index := range entries {
			entries[ index ].Trick , entries[ index ].Winner , entries[ index ].Points = 0 , -1 , 0
		}
	}
}
