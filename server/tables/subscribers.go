package tables

import (
	time "time"
)

// onlineGrace is how long someone still counts as at the table after their
// last stream closed. Every stream is ended on purpose every ~20 seconds and
// the browser reconnects about a second later; without this, each player
// would flash "away" to everyone else three times a minute. A var so the
// tests can shorten it.
var onlineGrace = 10 * time.Second

// Subscriber is one open live-update stream. Notify is a doorbell, not a
// queue: it holds at most one pending ring, so a slow phone that misses three
// changes gets one wake-up and then reads the latest snapshot. Nothing is
// ever "missed" because nothing but "something changed" is ever sent through
// it -- the snapshot is read fresh each time.
type Subscriber struct {
	UserID uint64 // 0 for someone watching without an account
	// Admin marks an admin's stream. An admin who is not seated watches
	// invisibly: they are left out of the watcher count everyone sees.
	Admin  bool
	Notify chan struct{}
}

func newSubscriber( user_id uint64 , admin bool ) ( subscriber *Subscriber ) {
	subscriber = &Subscriber{ UserID: user_id , Admin: admin , Notify: make( chan struct{} , 1 ) }
	return
}

func ( subscriber *Subscriber ) ring() {
	select {
	case subscriber.Notify <- struct{}{}:
	default:
	}
}

// SubscribeTable opens a stream on a table. Opening it is also how an invited
// player "arrives": the seat stops showing as held for them.
func ( hub *Hub ) SubscribeTable( id string , user_id uint64 , admin bool ) ( subscriber *Subscriber , err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	subscriber = newSubscriber( user_id , admin )

	table.mu.Lock()
	table.subscribers[ subscriber ] = struct{}{}
	others := table.subscriberList()
	pending := false
	if seat := table.seatOf( user_id ); user_id != 0 && seat >= 0 && table.Seats[ seat ].Pending {
		pending = true
	}
	table.mu.Unlock()

	if pending {
		hub.mutate( id , func( table *Table ) ( mutate_err error ) {
			if seat := table.seatOf( user_id ); seat >= 0 {
				table.Seats[ seat ].Pending = false
			}
			return
		} )
		return
	}
	// Someone came online: everyone's seat badges should say so.
	for _ , other := range others {
		other.ring()
	}
	return
}

// UnsubscribeTable closes a stream. Nobody is told straight away: the person
// counts as online for onlineGrace more (see online), which is long enough
// for a routine reconnect. Only once that has passed are the others rung, so
// their screens show the seat as away if it really is.
func ( hub *Hub ) UnsubscribeTable( id string , subscriber *Subscriber ) {
	table := hub.table( id )
	if table == nil { return }
	table.mu.Lock()
	delete( table.subscribers , subscriber )
	if subscriber.UserID != 0 {
		if table.left == nil { table.left = map[ uint64 ]time.Time{} }
		table.left[ subscriber.UserID ] = time.Now()
	}
	table.mu.Unlock()
	time.AfterFunc( onlineGrace + 100*time.Millisecond , func() {
		table.mu.Lock()
		others := table.subscriberList()
		table.mu.Unlock()
		for _ , other := range others {
			other.ring()
		}
	} )
}

func ( hub *Hub ) SubscribeLobby( user_id uint64 ) ( subscriber *Subscriber ) {
	subscriber = newSubscriber( user_id , false )
	hub.mu.Lock()
	hub.lobby[ subscriber ] = struct{}{}
	hub.mu.Unlock()
	return
}

func ( hub *Hub ) UnsubscribeLobby( subscriber *Subscriber ) {
	hub.mu.Lock()
	delete( hub.lobby , subscriber )
	hub.mu.Unlock()
}

func ( hub *Hub ) notifyLobby() {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for subscriber := range hub.lobby {
		subscriber.ring()
	}
}

// watchers counts the streams of people watching rather than playing,
// leaving out admins, who watch invisibly. Called with table.mu held.
func ( table *Table ) watchers() ( count int ) {
	for subscriber := range table.subscribers {
		if subscriber.Admin { continue }
		if subscriber.UserID != 0 && table.seatOf( subscriber.UserID ) >= 0 { continue }
		count++
	}
	return
}

// subscriberList copies the set so it can be rung after the table lock is
// released. Called with table.mu held.
func ( table *Table ) subscriberList() ( result []*Subscriber ) {
	result = make( []*Subscriber , 0 , len( table.subscribers ) )
	for subscriber := range table.subscribers {
		result = append( result , subscriber )
	}
	return
}
