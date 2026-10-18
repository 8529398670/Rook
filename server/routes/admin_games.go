package routes

import (
	strconv "strconv"
	strings "strings"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
	security "rook/server/security"
)

// Admin powers over games: step back and forth through a table's moves, put
// it on hold, decide who may see every hand, and the app-wide game settings
// and sign-up links. All of these are registered under RequireAdmin.

type resetRequest struct {
	To        int    `json:"to"`
	CSRFToken string `json:"csrf_token"`
}

type observersRequest struct {
	SeeAll    bool   `json:"see_all"`
	CSRFToken string `json:"csrf_token"`
}

type settingsRequest struct {
	models.AppSettings
	CSRFToken string `json:"csrf_token"`
}

type signupLinkRequest struct {
	Label     string `json:"label"`
	MaxUses   int    `json:"max_uses"`
	Days      int    `json:"days"`
	CSRFToken string `json:"csrf_token"`
}

type setCanHintRequest struct {
	CanHint   bool   `json:"can_hint"`
	CSRFToken string `json:"csrf_token"`
}

func ( handlers *Handlers ) GetHistory( c fiber.Ctx ) ( err error ) {
	history , history_err := handlers.Hub.History( c.Params( "table_id" ) )
	if history_err != nil {
		err = handlers.tableError( c , history_err )
		return
	}
	err = c.JSON( history )
	return
}

// historyStep runs one of undo / redo / resume, which take no arguments.
func ( handlers *Handlers ) historyStep( c fiber.Ctx , step func( id string ) error ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , step( id ) )
	return
}

func ( handlers *Handlers ) Undo( c fiber.Ctx ) ( err error ) {
	err = handlers.historyStep( c , handlers.Hub.Undo )
	return
}

func ( handlers *Handlers ) Redo( c fiber.Ctx ) ( err error ) {
	err = handlers.historyStep( c , handlers.Hub.Redo )
	return
}

func ( handlers *Handlers ) Resume( c fiber.Ctx ) ( err error ) {
	err = handlers.historyStep( c , handlers.Hub.Resume )
	return
}

func ( handlers *Handlers ) ResetTo( c fiber.Ctx ) ( err error ) {
	var body resetRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.ResetTo( id , body.To ) )
	return
}

func ( handlers *Handlers ) SetTableObservers( c fiber.Ctx ) ( err error ) {
	var body observersRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.SetObserversSeeAll( id , body.SeeAll ) )
	return
}

func ( handlers *Handlers ) GetSettings( c fiber.Ctx ) ( err error ) {
	err = c.JSON( handlers.Hub.Settings() )
	return
}

func ( handlers *Handlers ) SaveSettings( c fiber.Ctx ) ( err error ) {
	var body settingsRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	if handlers.Hub.SetSettings( body.AppSettings ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( handlers.Hub.Settings() )
	return
}

func ( handlers *Handlers ) SetUserCanHint( c fiber.Ctx ) ( err error ) {
	var body setCanHintRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if models.SetUserCanHint( handlers.Store , user_id , body.CanHint ) != nil {
		err = notFound( c , "no such user" )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

func signupLinkRow( link *models.SignupLink ) ( row fiber.Map ) {
	row = fiber.Map{
		"id":         link.ID,
		"label":      link.Label,
		"max_uses":   link.MaxUses,
		"used":       len( link.Accounts ),
		"remaining":  link.Remaining(),
		"live":       link.Live(),
		"expires_at": link.ExpiresAt,
		"revoked":    link.RevokedAt != nil,
	}
	return
}

func ( handlers *Handlers ) ListSignupLinks( c fiber.Ctx ) ( err error ) {
	links , list_err := models.ListSignupLinks( handlers.Store )
	if list_err != nil {
		err = serverError( c )
		return
	}
	rows := []fiber.Map{}
	for _ , link := range links {
		rows = append( rows , signupLinkRow( link ) )
	}
	err = c.JSON( rows )
	return
}

// CreateSignupLink returns the link's credential once, like a login link: it
// is stored only as a hash and cannot be shown again.
func ( handlers *Handlers ) CreateSignupLink( c fiber.Ctx ) ( err error ) {
	var body signupLinkRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	label := strings.TrimSpace( body.Label )
	if label == "" || len( label ) > 80 || body.MaxUses < 1 || body.MaxUses > 1000 || body.Days < 1 || body.Days > 365 {
		err = badRequest( c , "bad_signup_link" )
		return
	}
	credential , link , issue_err := models.IssueSignupLink( handlers.Store , label , body.MaxUses , time.Duration( body.Days )*24*time.Hour , security.UserFrom( c ).ID )
	if issue_err != nil {
		err = serverError( c )
		return
	}
	row := signupLinkRow( link )
	row[ "credential" ] = credential
	err = c.Status( fiber.StatusCreated ).JSON( row )
	return
}

func ( handlers *Handlers ) RevokeSignupLink( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	if models.RevokeSignupLink( handlers.Store , c.Params( "link_id" ) ) != nil {
		err = notFound( c , "no such link" )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}
