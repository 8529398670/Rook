// Package tables holds the live card tables: who sits where, the game in
// progress, chat, and the open live-update streams. The rules themselves live
// in server/game; this package is the part with locks, timers, and a
// database -- everything game deliberately does not touch.
//
// Every change to a table goes through Hub.mutate, which is the one place
// that bumps the version, saves the table, moves any computer player that is
// now up, and wakes the streams. A handler cannot forget one of those steps
// because it never does them itself.
package tables

import (
	strings "strings"
	sync "sync"
	time "time"
	utf8 "unicode/utf8"

	game "rook/server/game"
	models "rook/server/models"
)

// Error is a machine-readable refusal, like game.Error: the frontend turns
// the code into a sentence from language.yaml.
type Error string

func ( e Error ) Error() ( result string ) {
	result = string( e )
	return
}

const (
	ErrNoSuchTable    Error = "no_such_table"
	ErrBadSeat        Error = "bad_seat"
	ErrSeatTaken      Error = "seat_taken"
	ErrAlreadySeated  Error = "already_seated"
	ErrNotSeated      Error = "not_seated"
	ErrNotHost        Error = "not_host"
	ErrSeatsNotFull   Error = "seats_not_full"
	ErrGameInProgress Error = "game_in_progress"
	ErrPaused         Error = "game_paused"
	ErrNoGame         Error = "no_game"
	ErrTooManyTables  Error = "too_many_tables"
	ErrBadName        Error = "bad_table_name"
	ErrBadChat        Error = "bad_chat_message"
	ErrCannotInvite   Error = "cannot_invite"
	ErrNotABot        Error = "not_a_bot"
	ErrHeld           Error = "game_held"
	ErrNotHeld        Error = "not_held"
	ErrNothingToUndo  Error = "nothing_to_undo"
	ErrNothingToRedo  Error = "nothing_to_redo"
	ErrBadCursor      Error = "bad_history_point"
	ErrBadAction      Error = "bad_action"
	ErrNoSuchLink     Error = "no_such_link"
	ErrSeatsFull      Error = "seats_full"
	ErrNoRequest      Error = "no_such_request"
	ErrTooManyRequests Error = "too_many_requests"
	ErrSeatChanged    Error = "seat_changed"
	ErrNotAPlayer     Error = "not_a_player"
	ErrBadSeatName    Error = "bad_seat_name"
	ErrNoSuchGame     Error = "no_such_game"
	ErrBadGuestName   Error = "bad_guest_name"

	// errStale is internal: a delayed bot move found the table had moved on.
	errStale Error = "stale"
)

const (
	KindEmpty = ""
	KindHuman = "human"
	KindBot   = "bot"

	StatusWaiting = "waiting" // seating people, no cards dealt
	StatusPlaying = "playing"
	StatusPaused  = "paused" // a seat emptied mid-game; waits for someone to sit
	StatusHeld    = "held"   // an admin stepped through the history; waits for Resume
	StatusOver    = "over"

	maxNameLength = 40
	maxChatLength = 280
	maxGuestNameLength = 24
	maxChatLines  = 50
	viewChatLines = 40
	maxRequests   = 6
)

type Seat struct {
	Kind   string `json:"kind"`
	UserID uint64 `json:"user_id,omitempty"`
	Name   string `json:"name,omitempty"`
	// Bot numbers computer players 1-4 so each can have its own name in
	// language.yaml.
	Bot int `json:"bot,omitempty"`
	// Pending marks a seat held for someone who was invited and has not
	// opened the table yet.
	Pending bool `json:"pending,omitempty"`
	// The person's chosen avatar, copied here like Name so drawing a table
	// never needs a database read per seat.
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
	// Alias is what the host (or an admin) renamed this seat's player to,
	// for this table only. It goes with the seat: whoever leaves it takes
	// their own name back.
	Alias string `json:"alias,omitempty"`
}

// shownName is the seat's name as everyone at the table sees it.
func ( seat Seat ) shownName() ( result string ) {
	result = seat.Name
	if seat.Alias != "" { result = seat.Alias }
	return
}

// SeatRequest is someone watching who asked to take over a computer
// player's seat. The host (or an admin) says yes or no; until then the bot
// keeps playing. Who they are is copied here like a Seat, so the host's
// prompt can show their name and avatar without a database read.
type SeatRequest struct {
	UserID uint64    `json:"user_id"`
	Name   string    `json:"name"`
	Icon   string    `json:"icon,omitempty"`
	Color  string    `json:"color,omitempty"`
	Seat   int       `json:"seat"`
	At     time.Time `json:"at"`
}

// ChatLine is one message. Seat is -1 for someone watching: Watcher marks
// them, and Guest marks a watcher who is not signed in and typed their name.
type ChatLine struct {
	Seq     int       `json:"seq"`
	Seat    int       `json:"seat"`
	Name    string    `json:"name"`
	Text    string    `json:"text"`
	At      time.Time `json:"at"`
	Watcher bool      `json:"watcher,omitempty"`
	Guest   bool      `json:"guest,omitempty"`
}

// Table is saved whole, as JSON, after every change. The unexported fields
// are runtime-only: locks, streams, and the pending bot timer.
type Table struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	HostID    uint64       `json:"host_id"`
	Options   game.Options `json:"options"`
	Seats     [4]Seat      `json:"seats"`
	Game      *game.State  `json:"game,omitempty"`
	Chat      []ChatLine   `json:"chat"`
	ChatSeq   int          `json:"chat_seq"`
	Version   uint64       `json:"version"`
	// Requests to take over a bot's seat, waiting for the host.
	Requests []SeatRequest `json:"requests,omitempty"`

	// The move history (see history.go). Game is always Log[:Cursor]
	// replayed on top of Base; Base is only set for a game saved before
	// history was recorded.
	Base   *game.State `json:"base,omitempty"`
	Log    []Action    `json:"log,omitempty"`
	Cursor int         `json:"cursor"`
	Held   bool        `json:"held,omitempty"`

	// Shareable links: JoinCode seats a signed-in person, WatchCode lets
	// anyone watch. Both are random and separate from ID, so sharing one
	// grants exactly that and nothing else.
	JoinCode  string `json:"join_code"`
	WatchCode string `json:"watch_code"`
	// ObserversSeeAll lets people watching this table see every hand. An
	// admin sets it; the app-wide setting can also turn it on everywhere.
	ObserversSeeAll bool `json:"observers_see_all,omitempty"`
	// Archives maps each game played here (by where it starts in Log) to
	// its row in the game history. See archive.go.
	Archives map[ string ]ArchiveRef `json:"archives,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`

	mu          sync.Mutex
	subscribers map[ *Subscriber ]struct{}
	bot_timer   *time.Timer
	// left is when each person's last stream closed, for online's grace.
	left map[ uint64 ]time.Time
}

func validTableName( name string ) ( result string , ok bool ) {
	result = strings.TrimSpace( name )
	ok = result != "" && utf8.RuneCountInString( result ) <= maxNameLength
	return
}

func ( table *Table ) status() ( result string ) {
	switch {
	case table.Game == nil:
		result = StatusWaiting
	case table.Held:
		result = StatusHeld
	case table.Game.Phase == game.PhaseGameOver:
		result = StatusOver
	case table.emptySeats() > 0:
		result = StatusPaused
	default:
		result = StatusPlaying
	}
	return
}

func ( table *Table ) emptySeats() ( count int ) {
	for _ , seat := range table.Seats {
		if seat.Kind == KindEmpty { count++ }
	}
	return
}

// seatToTake is true while someone watching could get into the game: a bot
// they could ask to replace, or an empty chair they could sit in.
func ( table *Table ) seatToTake() ( result bool ) {
	for _ , seat := range table.Seats {
		if seat.Kind == KindBot || seat.Kind == KindEmpty {
			result = true
			return
		}
	}
	return
}

func ( table *Table ) humans() ( count int ) {
	for _ , seat := range table.Seats {
		if seat.Kind == KindHuman { count++ }
	}
	return
}

func ( table *Table ) seatOf( user_id uint64 ) ( index int ) {
	index = -1
	for candidate , seat := range table.Seats {
		if seat.Kind == KindHuman && seat.UserID == user_id {
			index = candidate
			return
		}
	}
	return
}

// gameActive is true while a hand is being played out: the only time an
// emptied seat pauses the table.
func ( table *Table ) gameActive() ( result bool ) {
	result = table.Game != nil && table.Game.Phase != game.PhaseGameOver
	return
}

func ( table *Table ) nextBotNumber() ( number int ) {
	used := map[ int ]bool{}
	for _ , seat := range table.Seats {
		if seat.Kind == KindBot { used[ seat.Bot ] = true }
	}
	for number = 1; used[ number ]; number++ {
	}
	return
}

// ensureHost hands hosting to another seated person when the host has left,
// so a table never ends up with nobody able to deal.
func ( table *Table ) ensureHost() {
	if table.seatOf( table.HostID ) >= 0 { return }
	for _ , seat := range table.Seats {
		if seat.Kind == KindHuman && seat.Pending == false {
			table.HostID = seat.UserID
			return
		}
	}
}

func ( table *Table ) addChat( line ChatLine ) {
	table.ChatSeq++
	line.Seq , line.At = table.ChatSeq , time.Now().UTC()
	table.Chat = append( table.Chat , line )
	if len( table.Chat ) > maxChatLines {
		table.Chat = append( []ChatLine( nil ) , table.Chat[ len( table.Chat )-maxChatLines: ]... )
	}
}

// requestOf is the index of user_id's pending seat request, or -1.
func ( table *Table ) requestOf( user_id uint64 ) ( index int ) {
	index = -1
	for candidate , request := range table.Requests {
		if request.UserID == user_id {
			index = candidate
			return
		}
	}
	return
}

func ( table *Table ) dropRequest( index int ) {
	table.Requests = append( table.Requests[ :index:index ] , table.Requests[ index+1: ]... )
}

// pruneRequests forgets requests that can no longer be granted: the seat is
// not a bot any more, or the person found a seat some other way. Called from
// mutate after every change, so a stale prompt never reaches the host.
func ( table *Table ) pruneRequests() {
	kept := table.Requests[ :0 ]
	for _ , request := range table.Requests {
		if validSeat( request.Seat ) && table.Seats[ request.Seat ].Kind == KindBot && table.seatOf( request.UserID ) < 0 {
			kept = append( kept , request )
		}
	}
	if len( kept ) == 0 { kept = nil }
	table.Requests = kept
}

// fillSeat copies a person onto a seat: who they are and how they look.
func fillSeat( seat *Seat , user *models.User ) {
	seat.Kind = KindHuman
	seat.UserID = user.ID
	seat.Name = user.DisplayName
	seat.Icon = user.Icon
	seat.Color = user.Color
}

// online is true while the person has a stream open, or closed their last
// one less than onlineGrace ago -- a reconnect, not someone leaving.
func ( table *Table ) online( user_id uint64 ) ( result bool ) {
	for subscriber := range table.subscribers {
		if subscriber.UserID == user_id {
			result = true
			return
		}
	}
	if left , ok := table.left[ user_id ]; ok && time.Since( left ) < onlineGrace {
		result = true
	}
	return
}
