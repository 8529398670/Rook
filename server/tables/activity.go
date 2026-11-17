package tables

import (
	time "time"
)

// Idle: someone online who has not touched the app -- no click, tap, key or
// scroll -- for idleAfter. Open pages report activity (POST /api/activity,
// at most once a minute, see static/js/player.js); opening the app counts
// too. Kept in memory only: after a restart everyone starts out active.

// idleAfter is how long without activity before someone shows as idle.
var idleAfter = 5 * time.Minute

// Presence is what the player menu shows about someone: online or not, and
// since when they have been idle.
type Presence struct {
	Online bool `json:"online"`
	Idle   bool `json:"idle"`
	// IdleSince is the moment of their last activity, set only while idle.
	IdleSince time.Time `json:"idle_since,omitzero"`
	// IdleSeconds is how long ago that was, by the server's clock, so a
	// page whose clock is off still shows the right length of time.
	IdleSeconds int64 `json:"idle_seconds,omitempty"`
}

// Touch records activity from user_id: they are not idle.
func ( hub *Hub ) Touch( user_id uint64 ) {
	hub.global_mu.Lock()
	hub.active_at[ user_id ] = time.Now()
	hub.global_mu.Unlock()
}

// PresenceOf answers whether user_id is online, and if so whether idle.
func ( hub *Hub ) PresenceOf( user_id uint64 ) ( presence Presence ) {
	hub.global_mu.Lock()
	_ , open := hub.presence[ user_id ]
	presence.Online = open || hub.wasOnline( user_id )
	active , known := hub.active_at[ user_id ]
	hub.global_mu.Unlock()
	if presence.Online == false || known == false { return }
	if idle := time.Since( active ); idle >= idleAfter {
		presence.Idle = true
		presence.IdleSince = active.UTC()
		presence.IdleSeconds = int64( idle / time.Second )
	}
	return
}
