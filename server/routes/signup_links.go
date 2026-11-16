package routes

import (
	strings "strings"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"

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
	Label     string `json:"label,omitempty"`
	MaxUses   int    `json:"max_uses,omitempty"`
	Days      int    `json:"days,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
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

// @route("GET /api/signup-links", summary="List signup links", tag="Sign-up links", access=SESSION_ONLY, use=[handlers.RequireInviter],
// )
func ( handlers *Handlers ) ListSignupLinks( c fiber.Ctx , in *apidoc.None ) ( err error ) {
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
//
// @route("POST /api/signup-links", summary="Create signup link", tag="Sign-up links", access=SESSION_ONLY, use=[handlers.RequireInviter],
// )
func ( handlers *Handlers ) CreateSignupLink( c fiber.Ctx , in *createSignupLinkIn ) ( err error ) {
	body := in.signupLinkRequest
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
//
// @route("POST /api/signup-links/{link_id}/revoke", summary="Revoke signup link", tag="Sign-up links", access=SESSION_ONLY, use=[handlers.RequireInviter],
// )
func ( handlers *Handlers ) RevokeSignupLink( c fiber.Ctx , in *revokeSignupLinkIn ) ( err error ) {
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

// DeleteSignupLink removes anyone's link, live or dead, so it drops off the
// list. The owner only (RequireOwner on the route).
//
// @route("POST /api/signup-links/{link_id}/delete", summary="Delete signup link", tag="Sign-up links", access=SESSION_ONLY, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) DeleteSignupLink( c fiber.Ctx , in *deleteSignupLinkIn ) ( err error ) {
	link , get_err := models.GetSignupLink( handlers.Store , c.Params( "link_id" ) )
	if get_err != nil {
		err = notFound( c , "no such link" )
		return
	}
	if models.DeleteSignupLink( handlers.Store , link.ID ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}
