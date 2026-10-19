package routes

import (
	strings "strings"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
	security "rook/server/security"
)

// Managing multi-use sign-up links (see models/signup_link.go; signing up
// through one is signup.go). Anyone who may invite can make them, since a
// sign-up link is the same power as inviting from an empty seat, just for
// a group at once. Admins see and can switch off everyone's; anyone else
// only their own.
//
// Registered with RequireSession: a link mints accounts, which a script
// holding an API key has no business doing.

type signupLinkRequest struct {
	Label     string `json:"label"`
	MaxUses   int    `json:"max_uses"`
	Days      int    `json:"days"`
	CSRFToken string `json:"csrf_token"`
}

// RequireInviter refuses anyone who may not bring new people in.
func ( handlers *Handlers ) RequireInviter( c fiber.Ctx ) ( err error ) {
	if security.UserFrom( c ).MayInvite() == false {
		err = forbidden( c , "forbidden" )
		return
	}
	err = c.Next()
	return
}

func ( handlers *Handlers ) signupLinkRow( link *models.SignupLink ) ( row fiber.Map ) {
	row = fiber.Map{
		"id":         link.ID,
		"label":      link.Label,
		"max_uses":   link.MaxUses,
		"used":       len( link.Accounts ),
		"remaining":  link.Remaining(),
		"live":       link.Live(),
		"expires_at": link.ExpiresAt,
		"revoked":    link.RevokedAt != nil,
		"created_by": link.CreatedBy,
	}
	if creator , get_err := models.GetUser( handlers.Store , link.CreatedBy ); get_err == nil {
		row[ "created_by_name" ] = creator.DisplayName
		// A link outlives its maker's permission only on paper: it stops
		// working the moment they lose it (see LoadSignupLink).
		if creator.Disabled() || creator.MayInvite() == false { row[ "live" ] = false }
	} else {
		row[ "live" ] = false
	}
	return
}

func ( handlers *Handlers ) ListSignupLinks( c fiber.Ctx ) ( err error ) {
	user := security.UserFrom( c )
	var links []*models.SignupLink
	var list_err error
	if user.IsAdmin() {
		links , list_err = models.ListSignupLinks( handlers.Store )
	} else {
		links , list_err = models.ListSignupLinksBy( handlers.Store , user.ID )
	}
	if list_err != nil {
		err = serverError( c )
		return
	}
	rows := []fiber.Map{}
	for _ , link := range links {
		rows = append( rows , handlers.signupLinkRow( link ) )
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
	row := handlers.signupLinkRow( link )
	row[ "credential" ] = credential
	err = c.Status( fiber.StatusCreated ).JSON( row )
	return
}

// RevokeSignupLink switches a link off. Someone else's link answers the
// same as no link at all, so ids cannot be probed.
func ( handlers *Handlers ) RevokeSignupLink( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	user := security.UserFrom( c )
	link , get_err := models.GetSignupLink( handlers.Store , c.Params( "link_id" ) )
	if get_err != nil || ( user.IsAdmin() == false && link.CreatedBy != user.ID ) {
		err = notFound( c , "no such link" )
		return
	}
	if models.RevokeSignupLink( handlers.Store , link.ID ) != nil {
		err = notFound( c , "no such link" )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}
