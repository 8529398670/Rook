package tables

import (
	sort "sort"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

type SeatInfo struct {
	Kind    string `json:"kind"`
	UserID  uint64 `json:"user_id,omitempty"`
	Name    string `json:"name"`
	Bot     int    `json:"bot"`
	Online  bool   `json:"online"`
	Pending bool   `json:"pending"`
	Renamed bool   `json:"renamed"`
	Host    bool   `json:"host"`
	Icon    string `json:"icon"`
	Color   string `json:"color"`
}

// RequestInfo is one pending "let me take over this bot" request, shown to
// the host and admins so they can answer it.
type RequestInfo struct {
	UserID uint64 `json:"user_id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
	Seat   int    `json:"seat"`
}

// Viewer is who is asking. User is nil for someone watching through a watch
// link without an account. Admin is the request's *effective* role, so an
// admin's user-scoped API key does not get admin sight.
type Viewer struct {
	User  *models.User
	Admin bool
}

// ViewOptions is what a watcher asked to see. A seated player always sees
// from their own seat with only their own hand; these only apply to people
// watching.
type ViewOptions struct {
	Perspective int  // whose seat to watch from; out of range means seat 0
	AllHands    bool // show every hand, if this watcher may
}

// TableView is what one person sees of a table. The game part comes from
// game.ViewFor, which is where hands are redacted.
type TableView struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Status    string       `json:"status"`
	Options   game.Options `json:"options"`
	Version   uint64       `json:"version"`
	You       int          `json:"you"`
	IsHost    bool         `json:"is_host"`
	CanInvite bool         `json:"can_invite"`
	CanHint   bool         `json:"can_hint"`
	Seats     [4]SeatInfo  `json:"seats"`
	Game      *game.View   `json:"game"`
	Chat      []ChatLine   `json:"chat"`
	// ChatOff: an admin has switched table chat off everywhere.
	ChatOff   bool         `json:"chat_off"`

	// Taking over a bot. Requests go to the host and admins only; MyRequest
	// is the seat the viewer asked for, or -1.
	Requests  []RequestInfo `json:"requests"`
	MyRequest int           `json:"my_request"`

	// Watching. Perspective is whose seat the screen is drawn from: your own
	// when you play, the chosen one when you watch.
	Perspective int  `json:"perspective"`
	Observing   bool `json:"observing"`
	Signed      bool `json:"signed_in"`
	CanSeeAll   bool `json:"can_see_all"`
	AllHands    bool `json:"all_hands"`
	// HandsHidden: this watcher could still sit down (take over a bot or an
	// empty chair), so they are shown no cards that are not on the table.
	HandsHidden bool `json:"hands_hidden"`
	Watchers    int  `json:"watchers"`

	// Admin tools.
	IsAdmin         bool `json:"is_admin"`
	Held            bool `json:"held"`
	Cursor          int  `json:"cursor"`
	Moves           int  `json:"moves"`
	CanUndo         bool `json:"can_undo"`
	CanRedo         bool `json:"can_redo"`
	ObserversSeeAll bool `json:"observers_see_all"`

	// Shareable links, given to the people at the table and to admins.
	JoinCode  string `json:"join_code,omitempty"`
	WatchCode string `json:"watch_code,omitempty"`
}

// Summary is a lobby row: enough to choose a table, nothing about the cards.
type Summary struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Status     string       `json:"status"`
	Options    game.Options `json:"options"`
	Seats      [4]SeatInfo  `json:"seats"`
	You        int          `json:"you"`
	HandNumber int          `json:"hand_number"`
	Scores     [2]int       `json:"scores"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// seatInfo is called with the table locked.
func ( table *Table ) seatInfo() ( seats [4]SeatInfo ) {
	for index , seat := range table.Seats {
		seats[ index ] = SeatInfo{
			Kind:    seat.Kind,
			UserID:  seat.UserID,
			Name:    seat.shownName(),
			Renamed: seat.Alias != "",
			Bot:     seat.Bot,
			Pending: seat.Pending,
			Icon:    seat.Icon,
			Color:   seat.Color,
			Host:    seat.Kind == KindHuman && seat.UserID == table.HostID,
			Online:  seat.Kind == KindBot || ( seat.Kind == KindHuman && table.online( seat.UserID ) ),
		}
	}
	return
}

func ( hub *Hub ) View( id string , viewer Viewer , options ViewOptions ) ( view TableView , err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	settings := hub.Settings()
	table.mu.Lock()
	defer table.mu.Unlock()

	you := -1
	if viewer.User != nil { you = table.seatOf( viewer.User.ID ) }
	perspective := you
	observing := you < 0
	if observing {
		perspective = options.Perspective
		if perspective < 0 || perspective > 3 { perspective = 0 }
	}
	// Someone signed in and watching who could still sit down -- there is a
	// bot to take over, or an empty chair -- sees no hidden cards at all:
	// not the watched seat's hand, not every hand, whatever the settings.
	// Otherwise they could study everyone's cards and then take a seat.
	// People on a watch link without an account can never sit, and admins
	// are trusted, so for them nothing changes.
	hands_hidden := observing && viewer.User != nil && viewer.Admin == false && table.gameActive() && table.seatToTake()
	// Every hand at once is for watchers only -- never for someone in the
	// game -- and only when an admin allowed it, here or app-wide. Admins
	// themselves may always look.
	can_see_all := observing && hands_hidden == false && ( viewer.Admin || table.ObserversSeeAll || settings.ObserversSeeAll )

	view = TableView{
		ID:              table.ID,
		Name:            table.Name,
		Status:          table.status(),
		Options:         table.Options,
		Version:         table.Version,
		You:             you,
		Seats:           table.seatInfo(),
		Chat:            []ChatLine{},
		Requests:        []RequestInfo{},
		MyRequest:       -1,
		Perspective:     perspective,
		Observing:       observing,
		Signed:          viewer.User != nil,
		CanSeeAll:       can_see_all,
		AllHands:        can_see_all && options.AllHands,
		HandsHidden:     hands_hidden,
		Watchers:        table.watchers(),
		IsAdmin:         viewer.Admin,
		Held:            table.Held,
		ObserversSeeAll: table.ObserversSeeAll,
		ChatOff:         settings.TableChatOff,
	}
	if viewer.User != nil {
		view.IsHost = table.HostID == viewer.User.ID
		view.CanInvite = viewer.User.MayInvite()
		view.CanHint = viewer.User.CanHint && you >= 0
		if index := table.requestOf( viewer.User.ID ); index >= 0 {
			view.MyRequest = table.Requests[ index ].Seat
		}
	}
	if view.IsHost || viewer.Admin {
		for _ , request := range table.Requests {
			view.Requests = append( view.Requests , RequestInfo{
				UserID: request.UserID , Name: request.Name , Icon: request.Icon , Color: request.Color , Seat: request.Seat ,
			} )
		}
	}
	if viewer.Admin {
		view.Cursor = table.Cursor
		view.Moves = len( table.Log )
		view.CanUndo = table.Cursor > 0
		view.CanRedo = table.Cursor < len( table.Log )
	}
	if you >= 0 || viewer.Admin {
		view.JoinCode = table.JoinCode
		view.WatchCode = table.WatchCode
	}
	if table.Game != nil {
		game_view := table.Game.ViewFor( perspective )
		if observing {
			// Watching a seat shows its cards, never the right to play them.
			game_view.Legal = []string{}
		}
		if hands_hidden {
			game_view.Hand , game_view.PickedUp = []string{} , []string{}
		}
		if view.AllHands {
			game_view.Hands = table.Game.AllHands()
		}
		view.Game = &game_view
	}
	chat := table.Chat
	if len( chat ) > viewChatLines {
		chat = chat[ len( chat )-viewChatLines: ]
	}
	view.Chat = append( view.Chat , chat... )
	return
}

// List is the lobby: your own tables first, then ones with an open seat, then
// the rest, newest activity first within each group.
func ( hub *Hub ) List( user *models.User ) ( summaries []Summary ) {
	hub.mu.RLock()
	tables := make( []*Table , 0 , len( hub.tables ) )
	for _ , table := range hub.tables {
		tables = append( tables , table )
	}
	hub.mu.RUnlock()

	summaries = make( []Summary , 0 , len( tables ) )
	for _ , table := range tables {
		table.mu.Lock()
		summary := Summary{
			ID:        table.ID,
			Name:      table.Name,
			Status:    table.status(),
			Options:   table.Options,
			Seats:     table.seatInfo(),
			You:       table.seatOf( user.ID ),
			UpdatedAt: table.UpdatedAt,
		}
		if table.Game != nil {
			summary.HandNumber = table.Game.HandNumber
			summary.Scores = table.Game.Scores
		}
		table.mu.Unlock()
		summaries = append( summaries , summary )
	}

	group := func( summary Summary ) ( result int ) {
		switch {
		case summary.You >= 0:
			result = 0
		case summary.Status == StatusWaiting || summary.Status == StatusPaused:
			result = 1
		default:
			result = 2
		}
		return
	}
	sort.SliceStable( summaries , func( i int , j int ) bool {
		if group( summaries[ i ] ) != group( summaries[ j ] ) {
			return group( summaries[ i ] ) < group( summaries[ j ] )
		}
		return summaries[ i ].UpdatedAt.After( summaries[ j ].UpdatedAt )
	} )
	return
}
