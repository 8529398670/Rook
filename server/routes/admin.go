package routes

import (
	errors "errors"
	strconv "strconv"
	strings "strings"
	time "time"

	fiber "github.com/gofiber/fiber/v3"


	models "rook/server/models"
	security "rook/server/security"
)

type createUserRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role,omitempty"`
	// AccountExpiresAt is when the new account stops working (portal
	// contract v2, account_expiry), omitted for never. expires_in_minutes,
	// in loginLinkRequest, stays the login link's own expiry.
	AccountExpiresAt *time.Time `json:"account_expires_at,omitempty"`
	CSRFToken        string     `json:"csrf_token,omitempty"`
}

type csrfOnlyRequest struct {
	CSRFToken string `json:"csrf_token,omitempty"`
}

type setCanInviteRequest struct {
	CanInvite bool   `json:"can_invite,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type setDisabledRequest struct {
	Disabled  bool   `json:"disabled,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type listUsersIn struct {
	Q      string `json:"q,omitempty" query:"q"`
	Limit  int    `json:"limit,omitempty" query:"limit"`
	Offset int    `json:"offset,omitempty" query:"offset"`
}

// listUsersMax is the most rows one page of GET /api/admin/users holds.
const listUsersMax = 500

// ListUsers is every account, a page at a time: users, next (the offset of
// the following page, null on the last) and total, the shape the project
// portal reads. q keeps the accounts whose name has it in, ignoring case.
//
// @route("GET /api/admin/users", summary="List users", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
//     description="Accounts, a page at a time, including disabled ones. q narrows by name; limit is "
//         "1-500 (default 500). next is the offset of the following page, or null on the last one.",
// )
func ( handlers *Handlers ) ListUsers( c fiber.Ctx , in *listUsersIn ) ( err error ) {
	limit := in.Limit
	if limit == 0 { limit = listUsersMax }
	if limit < 1 || limit > listUsersMax || in.Offset < 0 {
		err = badRequest( c , "limit must be 1-500 and offset 0 or more" )
		return
	}
	users , list_err := models.ListUsers( handlers.Store )
	if list_err != nil {
		err = serverError( c )
		return
	}
	super_id , super_err := models.SuperAdminID( handlers.Store )
	if super_err != nil {
		err = serverError( c )
		return
	}
	query := strings.ToLower( strings.TrimSpace( in.Q ) )
	rows := []fiber.Map{}
	for _ , user := range users {
		if query != "" && strings.Contains( strings.ToLower( user.DisplayName ) , query ) == false { continue }
		rows = append( rows , fiber.Map{
			"id":           user.ID,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"disabled":     user.Disabled(),
			"expires_at":   user.ExpiresAt,
			"expired":      user.Expired(),
			"can_invite":   user.CanInvite,
			"can_hint":     user.CanHint,
			"can_undo":     user.CanUndo,
			"can_reshuffle": user.CanReshuffle,
			"icon":         user.Icon,
			"color":        user.Color,
			"superadmin":   user.ID == super_id,
			"owner":        user.ID == super_id,
		} )
	}
	total := len( rows )
	var next *int
	from := min( in.Offset , total )
	to := min( from + limit , total )
	if to < total { next = &to }
	err = c.JSON( fiber.Map{ "users": rows[ from:to ] , "next": next , "total": total } )
	return
}

// CreateUser is the signup process: an admin names someone, and gets back a
// one-time link to hand them. The link is returned as a path, not a full URL
// -- the browser knows the origin it is talking to, whereas the server behind
// a reverse proxy often does not, and a login link with the wrong host is
// worse than useless.
//
// An admin key may call it too (the project portal does, to let someone
// join). The link records the key, so revoking the key voids it while it is
// unused (models.VoidLinksMintedByKeyTx).
//
// @route("POST /api/admin/users", summary="Create user", tag="Admin", role=ADMIN, use=[handlers.RequireOwner], status=201,
//     description="Make a new account and its first one-time login link, to hand to the person. role is "
//         "admin, user or viewer. account_expires_at (RFC 3339, optional) is when the account stops "
//         "working; omit it for never. Answers with user_id and login_path.",
// )
func ( handlers *Handlers ) CreateUser( c fiber.Ctx , in *createUserIn ) ( err error ) {
	body := in.createUserRequest
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "display name must be 1-80 characters" )
		return
	}
	if models.ValidRole( body.Role ) == false {
		err = badRequest( c , "role must be admin, user or viewer" )
		return
	}
	ttl , ok , err := loginLinkTTL( c , in.loginLinkRequest , handlers.Config.LoginTokenTTL )
	if ok == false { return }

	user , create_err := models.CreateUserExpiring( handlers.Store , body.DisplayName , body.Role , body.AccountExpiresAt )
	if errors.Is( create_err , models.ErrExpiryPast ) {
		err = badRequest( c , create_err.Error() )
		return
	}
	if create_err != nil {
		err = serverError( c )
		return
	}
	credential , token_err := models.IssueLoginTokenByKey( handlers.Store , user.ID , ttl , security.ActorFrom( c ).KeyID() )
	if token_err != nil {
		err = serverError( c )
		return
	}

	reply := loginLinkReply( credential , ttl )
	reply[ "user_id" ] = user.ID
	err = c.Status( fiber.StatusCreated ).JSON( reply )
	return
}

// ReissueLogin covers the only real failure mode of this auth model: someone
// lost their cookie or their device, and there is no password reset to fall
// back on. An admin mints a fresh link instead. Any link the account had not
// used yet stops working (see models.IssueLoginToken).
//
// It is not a reset: sessions the account already has stay signed in. When
// the worry is that someone else holds the account's credentials, use
// RevokeAccess first, then reissue.
//
// An admin key may call it too (the project portal does, to open Rook for
// someone); the link records the key, as in CreateUser.
//
// @route("POST /api/admin/users/{user_id}/reissue-login", summary="Reissue login", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
//     description="A fresh one-time login link for an account, for someone who lost their device or their "
//         "link. Answers with user_id and login_path.",
// )
func ( handlers *Handlers ) ReissueLogin( c fiber.Ctx , in *reissueLoginIn ) ( err error ) {

	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if _ , get_err := models.GetUser( handlers.Store , user_id ); get_err != nil {
		err = notFound( c , "no such user" )
		return
	}
	// A key reissuing its own account's link would get round the Sign-in
	// links switch: that is POST /api/account/login-link's to decide.
	actor := security.ActorFrom( c )
	if actor.ViaAPIKey() && actor.User.ID == user_id {
		err = forbidden( c , "an API key cannot reissue its own account's link: use POST /api/account/login-link" )
		return
	}
	ttl , ok , err := loginLinkTTL( c , in.loginLinkRequest , handlers.Config.LoginTokenTTL )
	if ok == false { return }

	credential , token_err := models.IssueLoginTokenByKey( handlers.Store , user_id , ttl , actor.KeyID() )
	if token_err != nil {
		err = serverError( c )
		return
	}
	reply := loginLinkReply( credential , ttl )
	reply[ "user_id" ] = user_id
	err = c.JSON( reply )
	return
}

// @route("POST /api/admin/users/{user_id}/disabled", summary="Set user disabled", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) SetUserDisabled( c fiber.Ctx , in *setUserDisabledIn ) ( err error ) {
	body := in.setDisabledRequest

	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if _ , get_err := models.GetUser( handlers.Store , user_id ); get_err != nil {
		err = notFound( c , "no such user" )
		return
	}

	// Locking yourself out is unrecoverable without shell access, so it is
	// worth one guard rail even though an admin could still disable a peer.
	if body.Disabled && user_id == security.UserFrom( c ).ID {
		err = badRequest( c , "you cannot disable your own account" )
		return
	}
	if body.Disabled && models.IsSuperAdmin( handlers.Store , user_id ) {
		err = badRequest( c , "superadmin" )
		return
	}

	// Disabling also deletes the account's sessions and unused login links,
	// in the same transaction -- see models.SetUserDisabled.
	if models.SetUserDisabled( handlers.Store , user_id , body.Disabled ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// RevokeAccess is the response to "this account may be compromised": every
// session, unused login link, and live API key it holds stops working at
// once. The account itself stays enabled; reissue a login link to let its
// owner back in. See models.RevokeAllCredentials for why this is separate
// from both disabling and reissuing.
//
// @route("POST /api/admin/users/{user_id}/revoke-access", summary="Revoke access", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) RevokeAccess( c fiber.Ctx , in *revokeAccessIn ) ( err error ) {
	var body csrfOnlyRequest
	user , ok , target_err := handlers.adminTarget( c , &body , func() string { return body.CSRFToken } )
	if ok == false {
		err = target_err
		return
	}
	// Same guard rail as disabling: revoking your own access signs you out
	// with no link to get back in.
	if user.ID == security.UserFrom( c ).ID {
		err = badRequest( c , "you cannot revoke your own access" )
		return
	}

	revoked , revoke_err := models.RevokeAllCredentials( handlers.Store , user.ID )
	if revoke_err != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "revoked": revoked } )
	return
}

// SetUserCanInvite hands one person the ability to bring new players in from
// a table's empty seat (see InviteToSeat). Admins can always invite, so the
// flag only means anything on a plain user.
//
// @route("POST /api/admin/users/{user_id}/can-invite", summary="Set user can invite", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) SetUserCanInvite( c fiber.Ctx , in *setUserCanInviteIn ) ( err error ) {
	body := in.setCanInviteRequest

	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if _ , get_err := models.GetUser( handlers.Store , user_id ); get_err != nil {
		err = notFound( c , "no such user" )
		return
	}
	if models.SetUserCanInvite( handlers.Store , user_id , body.CanInvite ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

type setExpiryRequest struct {
	ExpiresAt *time.Time `json:"expires_at"`
	CSRFToken string     `json:"csrf_token,omitempty"`
}

type setUserExpiryIn struct {
	pathUserID
	setExpiryRequest
}

// SetUserExpiry sets when an account stops working (portal contract v2,
// account_expiry). Who may do it lives in models.SetUserExpiry: never the
// owner's account. Rook has no housekeeping job, so an expired account is
// refused at every request (User.Disabled) rather than switched off later,
// and moving the expiry later lets it back in.
//
// @route("POST /api/admin/users/{user_id}/expiry", summary="Set when an account expires", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
//     description="expires_at is when the account stops working (RFC 3339), or null for never. Once it "
//         "passes the account counts as disabled: sign-in, its sessions and its keys stop. It must be in "
//         "the future. The owner's account never expires.",
// )
func ( handlers *Handlers ) SetUserExpiry( c fiber.Ctx , in *setUserExpiryIn ) ( err error ) {
	var body setExpiryRequest
	user , ok , target_err := handlers.adminTarget( c , &body , func() string { return body.CSRFToken } )
	if ok == false {
		err = target_err
		return
	}
	updated , set_err := models.SetUserExpiry( handlers.Store , user.ID , security.UserFrom( c ).ID , body.ExpiresAt )
	switch {
	case errors.Is( set_err , models.ErrExpiryPast ):
		err = badRequest( c , set_err.Error() )
		return
	case errors.Is( set_err , models.ErrOwnerProtected ):
		err = forbidden( c , "the owner's account cannot expire" )
		return
	case errors.Is( set_err , models.ErrOwnerRequired ):
		err = forbidden( c , set_err.Error() )
		return
	case set_err != nil:
		err = serverError( c , set_err )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "expires_at": updated.ExpiresAt } )
	return
}

type setRoleRequest struct {
	Role      string `json:"role,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// AdminSetRole promotes someone to admin or makes them a plain user again.
// Any admin may do it to anyone, themselves included, except that the
// superadmin (see models/superadmin.go) always stays an admin. Nothing else
// needs telling: every request reads the role fresh, and API keys are capped
// at their owner's role on each use.
//
// @route("POST /api/admin/users/{user_id}/role", summary="Admin set role", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) AdminSetRole( c fiber.Ctx , in *adminSetRoleIn ) ( err error ) {
	var body setRoleRequest
	user , ok , target_err := handlers.adminTarget( c , &body , func() string { return body.CSRFToken } )
	if ok == false {
		err = target_err
		return
	}
	if models.ValidRole( body.Role ) == false {
		err = badRequest( c , "role must be admin, user or viewer" )
		return
	}
	if body.Role != models.RoleAdmin && models.IsSuperAdmin( handlers.Store , user.ID ) {
		err = badRequest( c , "superadmin" )
		return
	}
	if models.SetUserRole( handlers.Store , user.ID , body.Role ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

type adminRenameRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	CSRFToken   string `json:"csrf_token,omitempty"`
}

// adminTarget decodes an admin request about one user and loads that user:
// the CSRF check, the id in the path, and a 404 for nobody. ok is false when
// it has already answered.
func ( handlers *Handlers ) adminTarget( c fiber.Ctx , body any , csrf func() string ) ( user *models.User , ok bool , err error ) {
	if c.Bind().JSON( body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	if handlers.Guard.CheckCSRF( c , csrf() ) == false {
		err = forbidden( c , "invalid csrf token" )
		return
	}
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	user , get_err := models.GetUser( handlers.Store , user_id )
	if get_err != nil {
		err = notFound( c , "no such user" )
		return
	}
	ok = true
	return
}

// AdminRenameUser changes someone's name for them; their seats show it at
// once.
//
// @route("POST /api/admin/users/{user_id}/rename", summary="Admin rename user", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) AdminRenameUser( c fiber.Ctx , in *adminRenameUserIn ) ( err error ) {
	var body adminRenameRequest
	user , ok , target_err := handlers.adminTarget( c , &body , func() string { return body.CSRFToken } )
	if ok == false {
		err = target_err
		return
	}
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "bad_name" )
		return
	}
	if models.RenameUser( handlers.Store , user.ID , body.DisplayName ) != nil {
		err = serverError( c )
		return
	}
	handlers.refreshTables( user.ID )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// AdminSetAvatar picks someone's icon and color for them.
//
// @route("POST /api/admin/users/{user_id}/avatar", summary="Admin set avatar", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) AdminSetAvatar( c fiber.Ctx , in *adminSetAvatarIn ) ( err error ) {
	var body avatarRequest
	user , ok , target_err := handlers.adminTarget( c , &body , func() string { return body.CSRFToken } )
	if ok == false {
		err = target_err
		return
	}
	if models.SetUserAvatar( handlers.Store , user.ID , body.Icon , body.Color ) != nil {
		err = badRequest( c , "bad_avatar" )
		return
	}
	handlers.refreshTables( user.ID )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// DeleteUser removes an account for good, unlike disabling it: the row,
// every way to sign in as it, and its seats at open tables. Games already
// in the history keep the name it played under.
//
// @route("POST /api/admin/users/{user_id}/delete", summary="Delete user", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) DeleteUser( c fiber.Ctx , in *deleteUserIn ) ( err error ) {
	var body csrfOnlyRequest
	user , ok , target_err := handlers.adminTarget( c , &body , func() string { return body.CSRFToken } )
	if ok == false {
		err = target_err
		return
	}
	if user.ID == security.UserFrom( c ).ID {
		err = badRequest( c , "cannot_delete_self" )
		return
	}
	if models.IsSuperAdmin( handlers.Store , user.ID ) {
		err = badRequest( c , "superadmin" )
		return
	}
	if user.IsAdmin() && user.Disabled() == false {
		admins , count_err := models.CountActiveAdmins( handlers.Store )
		if count_err != nil {
			err = serverError( c )
			return
		}
		if admins <= 1 {
			err = badRequest( c , "last_admin" )
			return
		}
	}
	if models.DeleteUser( handlers.Store , user.ID ) != nil {
		err = serverError( c )
		return
	}
	handlers.Hub.ForgetUser( user.ID )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}
