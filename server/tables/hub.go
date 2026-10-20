package tables

import (
	json "encoding/json"
	errors "errors"
	log "log"
	sync "sync"
	atomic "sync/atomic"
	time "time"

	config "rook/server/config"
	db "rook/server/db"
	encryption "rook/server/encryption"
	game "rook/server/game"
	models "rook/server/models"
)

// Hub owns every table in memory. The database is the durable copy -- each
// change is written through before anyone is told about it -- so a restart
// reloads every game exactly where it stood, hands and all.
type Hub struct {
	store *db.Store
	cfg   *config.Config

	mu       sync.RWMutex
	tables   map[ string ]*Table
	settings models.AppSettings
	// trick_pause mirrors settings.TrickPause() for scheduleBot, which runs
	// with a table locked and so must not take mu (see View on lock order).
	trick_pause atomic.Int64
	lobby  map[ *Subscriber ]struct{}
	closed bool
	done   chan struct{}

	// The global chat and who is online (see global.go), under their own
	// lock so a busy chat never waits on the tables.
	global_mu     sync.Mutex
	global        map[ *Subscriber ]struct{}
	global_chat   []models.GlobalChatLine
	global_seq    int
	presence      map[ uint64 ]int
	presence_left map[ uint64 ]time.Time
}

func New( store *db.Store , cfg *config.Config ) ( hub *Hub , err error ) {
	hub = &Hub{
		store:  store,
		cfg:    cfg,
		tables: map[ string ]*Table{},
		lobby:  map[ *Subscriber ]struct{}{},
		done:   make( chan struct{} ),

		global:        map[ *Subscriber ]struct{}{},
		presence:      map[ uint64 ]int{},
		presence_left: map[ uint64 ]time.Time{},
	}

	if hub.settings , err = models.LoadSettings( store ); err != nil { return }
	hub.trick_pause.Store( int64( hub.settings.TrickPause() ) )
	if hub.global_chat , err = models.LoadGlobalChat( store ); err != nil { return }
	if count := len( hub.global_chat ); count > 0 { hub.global_seq = hub.global_chat[ count-1 ].Seq }

	stored , err := models.ListTables( store )
	if err != nil { return }
	for _ , record := range stored {
		table := &Table{}
		if decode_err := json.Unmarshal( record.Document , table ); decode_err != nil {
			// One unreadable table should not keep every other game from
			// loading. Leave it in the bucket for someone to look at.
			log.Printf( "[tables] skipping unreadable table %s: %v" , record.ID , decode_err )
			continue
		}
		table.subscribers = map[ *Subscriber ]struct{}{}
		upgraded := false
		// Tables saved before move history existed: start the history from
		// where the game stands, so undo works from here on.
		if table.Game != nil && len( table.Log ) == 0 && table.Base == nil {
			table.Base , _ = cloneState( table.Game )
			upgraded = true
		}
		if table.JoinCode == "" || table.WatchCode == "" {
			table.JoinCode , table.WatchCode = newCode() , newCode()
			upgraded = true
		}
		if upgraded { hub.persist( table ) }
		hub.tables[ table.ID ] = table
	}
	// Resume any computer player whose turn it was when the server stopped.
	for _ , table := range hub.tables {
		table.mu.Lock()
		hub.scheduleBot( table )
		table.mu.Unlock()
	}
	return
}

// Done closes when the hub shuts down, which is what ends every open stream
// promptly instead of leaving graceful shutdown waiting on them.
func ( hub *Hub ) Done() ( done <-chan struct{} ) {
	done = hub.done
	return
}

func ( hub *Hub ) Close() {
	hub.mu.Lock()
	if hub.closed {
		hub.mu.Unlock()
		return
	}
	hub.closed = true
	close( hub.done )
	tables := make( []*Table , 0 , len( hub.tables ) )
	for _ , table := range hub.tables {
		tables = append( tables , table )
	}
	hub.mu.Unlock()

	for _ , table := range tables {
		table.mu.Lock()
		if table.bot_timer != nil { table.bot_timer.Stop() }
		table.mu.Unlock()
	}
}

func ( hub *Hub ) table( id string ) ( table *Table ) {
	hub.mu.RLock()
	table = hub.tables[ id ]
	hub.mu.RUnlock()
	return
}

// mutate is the only way a table changes. fn runs under the table's lock;
// if it returns an error nothing else happens. Otherwise, in order: bots that
// only need to say "ready" do so, the version is bumped, the table is saved,
// the next bot move is scheduled, and every stream is woken.
func ( hub *Hub ) mutate( id string , fn func( table *Table ) error ) ( err error ) {
	hub.mu.RLock()
	closed := hub.closed
	hub.mu.RUnlock()
	if closed {
		err = errStale
		return
	}

	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}

	table.mu.Lock()
	if err = fn( table ); err != nil {
		table.mu.Unlock()
		return
	}
	table.pruneRequests()
	hub.settleBots( table )
	table.ensureHost()
	table.Version++
	table.UpdatedAt = time.Now().UTC()
	hub.syncArchive( table )
	abandoned := table.humans() == 0
	if abandoned == false {
		hub.persist( table )
		hub.scheduleBot( table )
	} else {
		hub.archiveAbandoned( table )
		if table.bot_timer != nil { table.bot_timer.Stop() }
	}
	subscribers := table.subscriberList()
	table.mu.Unlock()

	if abandoned {
		// Nobody left but computer players: there is no one to finish the
		// game for, so the table goes rather than sitting in the lobby.
		hub.remove( id )
	}
	for _ , subscriber := range subscribers {
		subscriber.ring()
	}
	hub.notifyLobby()
	return
}

// persist is called with the table locked. A failed write is logged rather
// than returned: the move already happened in memory and everyone is about
// to see it, and the next change writes the whole table again anyway.
func ( hub *Hub ) persist( table *Table ) {
	document , err := json.Marshal( table )
	if err == nil {
		err = models.SaveTable( hub.store , table.ID , document )
	}
	if err != nil {
		log.Printf( "[tables] could not save table %s: %v" , table.ID , err )
	}
}

func ( hub *Hub ) remove( id string ) {
	hub.mu.Lock()
	delete( hub.tables , id )
	hub.mu.Unlock()
	if err := models.DeleteTable( hub.store , id ); err != nil {
		log.Printf( "[tables] could not delete table %s: %v" , id , err )
	}
}

// Create opens a table with its creator in seat 0 as host.
func ( hub *Hub ) Create( user *models.User , name string , options game.Options ) ( id string , err error ) {
	name , ok := validTableName( name )
	if ok == false {
		err = ErrBadName
		return
	}
	// The bird's house rules are the owner's app-wide choice, not the
	// creator's: whatever was sent, the table starts with the current ones.
	settings := hub.Settings()
	options.RookAnytime = settings.RookAnytime
	options.RookFirstTrick = settings.RookFirstTrickOff == false
	if err = options.Validate(); err != nil { return }

	hub.mu.Lock()
	if len( hub.tables ) >= hub.cfg.MaxTables {
		hub.mu.Unlock()
		err = ErrTooManyTables
		return
	}
	now := time.Now().UTC()
	table := &Table{
		ID:          encryption.GenerateURLToken( 9 ),
		Name:        name,
		HostID:      user.ID,
		Options:     options,
		Chat:        []ChatLine{},
		Version:     1,
		JoinCode:    newCode(),
		WatchCode:   newCode(),
		CreatedAt:   now,
		UpdatedAt:   now,
		subscribers: map[ *Subscriber ]struct{}{},
	}
	fillSeat( &table.Seats[ 0 ] , user )
	hub.tables[ table.ID ] = table
	hub.mu.Unlock()

	table.mu.Lock()
	hub.persist( table )
	table.mu.Unlock()
	hub.notifyLobby()
	id = table.ID
	return
}

func newCode() ( code string ) {
	code = encryption.GenerateURLToken( 12 )
	return
}

// Settings are the admin's app-wide game switches, kept in memory so every
// table view does not read the database for them.
func ( hub *Hub ) Settings() ( settings models.AppSettings ) {
	hub.mu.RLock()
	settings = hub.settings
	hub.mu.RUnlock()
	return
}

// SetSettings saves new settings and redraws every table, since "observers
// see all hands" changes what watchers are shown and either chat may have
// been switched on or off.
func ( hub *Hub ) SetSettings( settings models.AppSettings ) ( err error ) {
	if err = models.SaveSettings( hub.store , settings ); err != nil { return }
	hub.mu.Lock()
	hub.settings = settings
	hub.trick_pause.Store( int64( settings.TrickPause() ) )
	tables := make( []*Table , 0 , len( hub.tables ) )
	for _ , table := range hub.tables {
		tables = append( tables , table )
	}
	hub.mu.Unlock()
	for _ , table := range tables {
		table.mu.Lock()
		subscribers := table.subscriberList()
		table.mu.Unlock()
		for _ , subscriber := range subscribers {
			subscriber.ring()
		}
	}
	hub.notifyGlobal()
	return
}

// SetObserversSeeAll is the per-table version of the same switch.
func ( hub *Hub ) SetObserversSeeAll( id string , see_all bool ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		table.ObserversSeeAll = see_all
		return
	} )
	return
}

// SetTrickPause sets how long this table's finished tricks stay before they
// are swept away, or with 0 goes back to the app-wide default.
func ( hub *Hub ) SetTrickPause( id string , ms int ) ( err error ) {
	if models.ValidTrickPause( ms ) == false {
		err = ErrBadTrickPause
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		table.TrickPauseMS = ms
		return
	} )
	return
}

// ByCode finds the table a join or watch link points at.
func ( hub *Hub ) ByCode( code string , watch bool ) ( id string , err error ) {
	err = ErrNoSuchLink
	if code == "" { return }
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for _ , table := range hub.tables {
		table.mu.Lock()
		match := ( watch && table.WatchCode == code ) || ( watch == false && table.JoinCode == code )
		table.mu.Unlock()
		if match {
			id , err = table.ID , nil
			return
		}
	}
	return
}

// Join is what a join link does for someone signed in: take the first open
// seat if they have none and seats are open to change. With every chair
// taken but a bot at the table, it asks the host to let them take over the
// bot's seat. Otherwise it just opens the table, to watch.
func ( hub *Hub ) Join( code string , user *models.User ) ( id string , err error ) {
	if id , err = hub.ByCode( code , false ); err != nil { return }
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if table.seatOf( user.ID ) >= 0 || table.requestOf( user.ID ) >= 0 {
			fn_err = errStale // already seated or already asking: nothing to change
			return
		}
		status := table.status()
		if status == StatusWaiting || status == StatusPaused || status == StatusOver {
			for index := range table.Seats {
				if table.Seats[ index ].Kind == KindEmpty {
					table.Seats[ index ] = Seat{}
					fillSeat( &table.Seats[ index ] , user )
					return
				}
			}
		}
		// No free chair, but a computer player could make room: ask the
		// host to let them take over the first one.
		for index := range table.Seats {
			if table.Seats[ index ].Kind == KindBot {
				fn_err = table.addRequest( user , index )
				return
			}
		}
		fn_err = errStale
		return
	} )
	if err == errStale || err == ErrTooManyRequests { err = nil }
	return
}

// IsSeated reports whether user_id has a seat at table id.
func ( hub *Hub ) IsSeated( id string , user_id uint64 ) ( result bool ) {
	table := hub.table( id )
	if table == nil { return }
	table.mu.Lock()
	result = table.seatOf( user_id ) >= 0
	table.mu.Unlock()
	return
}

// RefreshUser copies a renamed or re-iconed person onto every seat they
// hold, so the change shows at their tables straight away -- and on the
// online list.
func ( hub *Hub ) RefreshUser( user *models.User ) {
	hub.notifyGlobal()
	hub.mu.RLock()
	ids := []string{}
	for id , table := range hub.tables {
		table.mu.Lock()
		if table.seatOf( user.ID ) >= 0 { ids = append( ids , id ) }
		table.mu.Unlock()
	}
	hub.mu.RUnlock()
	for _ , id := range ids {
		hub.mutate( id , func( table *Table ) ( fn_err error ) {
			if seat := table.seatOf( user.ID ); seat >= 0 {
				pending := table.Seats[ seat ].Pending
				fillSeat( &table.Seats[ seat ] , user )
				table.Seats[ seat ].Pending = pending
				// Auto play already on here follows a new level at once.
				if _ , on := table.AutoPlay[ user.ID ]; on && table.AutoPlayLevel != nil {
					table.AutoPlayLevel[ user.ID ] = AutoPlayLevelOf( user )
				}
			}
			return
		} )
	}
}

// ForgetUser clears a deleted account out of every table: their seat empties,
// exactly as if the host had removed them, and any request of theirs to take
// over a seat is dropped. A table left with nobody but computer players goes,
// as when the last person leaves.
func ( hub *Hub ) ForgetUser( user_id uint64 ) {
	hub.mu.RLock()
	ids := []string{}
	for id , table := range hub.tables {
		table.mu.Lock()
		if table.seatOf( user_id ) >= 0 || table.requestOf( user_id ) >= 0 { ids = append( ids , id ) }
		table.mu.Unlock()
	}
	hub.mu.RUnlock()
	for _ , id := range ids {
		hub.mutate( id , func( table *Table ) ( fn_err error ) {
			if seat := table.seatOf( user_id ); seat >= 0 { table.Seats[ seat ] = Seat{} }
			if index := table.requestOf( user_id ); index >= 0 { table.dropRequest( index ) }
			return
		} )
	}
}

// Purge removes tables that finished, or that nobody has touched, longer ago
// than the retention window. Called from main's hourly purge loop.
func ( hub *Hub ) Purge() ( removed int ) {
	cutoff := time.Now().UTC().Add( -hub.cfg.TableRetention )
	doomed := []string{}
	hub.mu.RLock()
	for id , table := range hub.tables {
		table.mu.Lock()
		stale := table.UpdatedAt.Before( cutoff )
		table.mu.Unlock()
		if stale { doomed = append( doomed , id ) }
	}
	hub.mu.RUnlock()

	for _ , id := range doomed {
		if table := hub.table( id ); table != nil {
			table.mu.Lock()
			if table.bot_timer != nil { table.bot_timer.Stop() }
			hub.archiveAbandoned( table )
			subscribers := table.subscriberList()
			table.mu.Unlock()
			hub.remove( id )
			for _ , subscriber := range subscribers {
				subscriber.ring()
			}
		}
	}
	if len( doomed ) > 0 { hub.notifyLobby() }
	removed = len( doomed )
	return
}

// IsUserError tells a handler whether err is a refusal to report to the
// player (400-class) or a genuine failure.
func IsUserError( err error ) ( code string , ok bool ) {
	var table_err Error
	if errors.As( err , &table_err ) && table_err != errStale {
		code , ok = string( table_err ) , true
		return
	}
	var game_err game.Error
	if errors.As( err , &game_err ) {
		code , ok = string( game_err ) , true
	}
	return
}
