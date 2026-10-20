package routes

import (
	sync "sync"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
	security "rook/server/security"
)

// Multi-use sign-up links (see models/signup_link.go). Everything here is
// public -- holding the link is the permission -- so every POST checks it
// came from this site, and the credential travels in the body, never the
// URL, so it stays out of access logs.

type signupRequest struct {
	Credential  string `json:"credential"`
	DisplayName string `json:"display_name"`
	UserID      uint64 `json:"user_id"`
	Pin         string `json:"pin"`
}

// pinGuard limits wrong PINs per account. Four digits is only ten thousand
// guesses, so without this a PIN is decoration: five misses lock that
// account's PIN for fifteen minutes. In memory, like the rate limiter --
// see middleware.RateLimit for what that means with more than one replica.
type pinGuard struct {
	mu       sync.Mutex
	failures map[ uint64 ]int
	until    map[ uint64 ]time.Time
}

var pins = &pinGuard{ failures: map[ uint64 ]int{} , until: map[ uint64 ]time.Time{} }

const (
	pinMaxFailures = 5
	pinLockout     = 15 * time.Minute
)

func ( guard *pinGuard ) locked( user_id uint64 ) ( result bool ) {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	result = time.Now().Before( guard.until[ user_id ] )
	return
}

func ( guard *pinGuard ) record( user_id uint64 , ok bool ) {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if ok {
		delete( guard.failures , user_id )
		return
	}
	guard.failures[ user_id ]++
	if guard.failures[ user_id ] >= pinMaxFailures {
		guard.until[ user_id ] = time.Now().Add( pinLockout )
		delete( guard.failures , user_id )
	}
}

// signupLink decodes the body, checks the request is same-site, and loads
// the link. A bad link answers 404 like any unknown thing.
func ( handlers *Handlers ) signupLink( c fiber.Ctx , body *signupRequest ) ( link *models.SignupLink , err error ) {
	if security.SameOrigin( c ) == false {
		err = forbidden( c , "cross-site request" )
		return
	}
	if c.Bind().JSON( body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	link , load_err := models.LoadSignupLink( handlers.Store , body.Credential )
	if load_err != nil {
		link = nil
		err = c.Status( fiber.StatusNotFound ).JSON( fiber.Map{ "error": "signup_link_invalid" } )
	}
	return
}

func ( handlers *Handlers ) SignupInfo( c fiber.Ctx ) ( err error ) {
	var body signupRequest
	link , err := handlers.signupLink( c , &body )
	if link == nil { return }
	err = c.JSON( fiber.Map{ "label": link.Label , "remaining": link.Remaining() } )
	return
}

// SignupMatch is the "did you mean...?": the accounts from this link whose
// names are close to what was typed.
func ( handlers *Handlers ) SignupMatch( c fiber.Ctx ) ( err error ) {
	var body signupRequest
	link , err := handlers.signupLink( c , &body )
	if link == nil { return }
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "bad_name" )
		return
	}
	matches , match_err := models.SignupMatches( handlers.Store , link , body.DisplayName )
	if match_err != nil {
		err = serverError( c )
		return
	}
	rows := []fiber.Map{}
	for _ , user := range matches {
		rows = append( rows , fiber.Map{
			"id": user.ID , "display_name": user.DisplayName , "icon": user.Icon , "color": user.Color , "has_pin": user.HasPin(),
		} )
	}
	err = c.JSON( fiber.Map{ "matches": rows , "can_create": link.Remaining() > 0 } )
	return
}

// SignupClaim signs in as an account this link created -- the same person
// on another device. Asks for the PIN if that person set one.
func ( handlers *Handlers ) SignupClaim( c fiber.Ctx ) ( err error ) {
	var body signupRequest
	link , err := handlers.signupLink( c , &body )
	if link == nil { return }
	user , get_err := models.GetUser( handlers.Store , body.UserID )
	if get_err != nil || link.Owns( body.UserID ) == false || user.Disabled() || user.IsAdmin() {
		err = c.Status( fiber.StatusNotFound ).JSON( fiber.Map{ "error": "no_such_account" } )
		return
	}
	if user.HasPin() {
		if pins.locked( user.ID ) {
			err = c.Status( fiber.StatusTooManyRequests ).JSON( fiber.Map{ "error": "pin_locked" } )
			return
		}
		ok := user.CheckPin( body.Pin )
		pins.record( user.ID , ok )
		if ok == false {
			code := "pin_wrong"
			if body.Pin == "" { code = "pin_needed" }
			err = c.Status( fiber.StatusForbidden ).JSON( fiber.Map{ "error": code } )
			return
		}
	}
	if handlers.startSession( c , user.ID ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "display_name": user.DisplayName } )
	return
}

// SignupCreate makes a new account through the link, if it has places left.
func ( handlers *Handlers ) SignupCreate( c fiber.Ctx ) ( err error ) {
	var body signupRequest
	link , err := handlers.signupLink( c , &body )
	if link == nil { return }
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "bad_name" )
		return
	}
	if link.Remaining() <= 0 {
		err = c.Status( fiber.StatusConflict ).JSON( fiber.Map{ "error": "signup_link_full" } )
		return
	}
	user , create_err := models.CreateUser( handlers.Store , body.DisplayName , models.RoleUser )
	if create_err != nil {
		err = serverError( c )
		return
	}
	if record_err := models.RecordSignup( handlers.Store , link.ID , user.ID ); record_err != nil {
		// Someone else took the last place a moment ago. Disable the account
		// just made rather than leave a person no link counts for.
		models.SetUserDisabled( handlers.Store , user.ID , true )
		err = c.Status( fiber.StatusConflict ).JSON( fiber.Map{ "error": "signup_link_full" } )
		return
	}
	if handlers.startSession( c , user.ID ) != nil {
		err = serverError( c )
		return
	}
	err = c.Status( fiber.StatusCreated ).JSON( fiber.Map{ "ok": true , "display_name": user.DisplayName } )
	return
}
