package routes

import (
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
	security "rook/server/security"
)

type createUserRequest struct {
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	CSRFToken   string `json:"csrf_token"`
}

type csrfOnlyRequest struct {
	CSRFToken string `json:"csrf_token"`
}

type setCanInviteRequest struct {
	CanInvite bool   `json:"can_invite"`
	CSRFToken string `json:"csrf_token"`
}

type setDisabledRequest struct {
	Disabled  bool   `json:"disabled"`
	CSRFToken string `json:"csrf_token"`
}

func ( handlers *Handlers ) ListUsers( c fiber.Ctx ) ( err error ) {
	users , list_err := models.ListUsers( handlers.Store )
	if list_err != nil {
		err = serverError( c )
		return
	}
	rows := []fiber.Map{}
	for _ , user := range users {
		rows = append( rows , fiber.Map{
			"id":           user.ID,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"disabled":     user.Disabled(),
			"can_invite":   user.CanInvite,
			"can_hint":     user.CanHint,
			"icon":         user.Icon,
			"color":        user.Color,
		} )
	}
	err = c.JSON( rows )
	return
}

// CreateUser is the signup process: an admin names someone, and gets back a
// one-time link to hand them. The link is returned as a path, not a full URL
// -- the browser knows the origin it is talking to, whereas the server behind
// a reverse proxy often does not, and a login link with the wrong host is
// worse than useless.
func ( handlers *Handlers ) CreateUser( c fiber.Ctx ) ( err error ) {
	var body createUserRequest
	if c.Bind().Body( &body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	if handlers.Guard.CheckCSRF( c , body.CSRFToken ) == false {
		err = forbidden( c , "invalid csrf token" )
		return
	}
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "display name must be 1-80 characters" )
		return
	}
	if models.ValidRole( body.Role ) == false {
		err = badRequest( c , "role must be admin or user" )
		return
	}

	user , create_err := models.CreateUser( handlers.Store , body.DisplayName , body.Role )
	if create_err != nil {
		err = serverError( c )
		return
	}
	credential , token_err := models.IssueLoginToken( handlers.Store , user.ID , handlers.Config.LoginTokenTTL )
	if token_err != nil {
		err = serverError( c )
		return
	}

	err = c.Status( fiber.StatusCreated ).JSON( fiber.Map{
		"user_id":            user.ID,
		"login_path":         "/login/" + credential,
		"expires_in_seconds": int( handlers.Config.LoginTokenTTL.Seconds() ),
	} )
	return
}

// ReissueLogin covers the only real failure mode of this auth model: someone
// lost their cookie or their device, and there is no password reset to fall
// back on. An admin mints a fresh link instead.
func ( handlers *Handlers ) ReissueLogin( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if c.Bind().Body( &body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	if handlers.Guard.CheckCSRF( c , body.CSRFToken ) == false {
		err = forbidden( c , "invalid csrf token" )
		return
	}

	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if _ , get_err := models.GetUser( handlers.Store , user_id ); get_err != nil {
		err = notFound( c , "no such user" )
		return
	}

	credential , token_err := models.IssueLoginToken( handlers.Store , user_id , handlers.Config.LoginTokenTTL )
	if token_err != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{
		"login_path":         "/login/" + credential,
		"expires_in_seconds": int( handlers.Config.LoginTokenTTL.Seconds() ),
	} )
	return
}

func ( handlers *Handlers ) SetUserDisabled( c fiber.Ctx ) ( err error ) {
	var body setDisabledRequest
	if c.Bind().Body( &body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	if handlers.Guard.CheckCSRF( c , body.CSRFToken ) == false {
		err = forbidden( c , "invalid csrf token" )
		return
	}

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

	if models.SetUserDisabled( handlers.Store , user_id , body.Disabled ) != nil {
		err = serverError( c )
		return
	}
	if body.Disabled {
		// Revoke immediately rather than waiting for the cookie to expire.
		models.DestroyAllSessionsForUser( handlers.Store , user_id )
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// SetUserCanInvite hands one person the ability to bring new players in from
// a table's empty seat (see InviteToSeat). Admins can always invite, so the
// flag only means anything on a plain user.
func ( handlers *Handlers ) SetUserCanInvite( c fiber.Ctx ) ( err error ) {
	var body setCanInviteRequest
	if c.Bind().Body( &body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	if handlers.Guard.CheckCSRF( c , body.CSRFToken ) == false {
		err = forbidden( c , "invalid csrf token" )
		return
	}

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

type adminRenameRequest struct {
	DisplayName string `json:"display_name"`
	CSRFToken   string `json:"csrf_token"`
}

// adminTarget decodes an admin request about one user and loads that user:
// the CSRF check, the id in the path, and a 404 for nobody. ok is false when
// it has already answered.
func ( handlers *Handlers ) adminTarget( c fiber.Ctx , body any , csrf func() string ) ( user *models.User , ok bool , err error ) {
	if c.Bind().Body( body ) != nil {
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
func ( handlers *Handlers ) AdminRenameUser( c fiber.Ctx ) ( err error ) {
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
func ( handlers *Handlers ) AdminSetAvatar( c fiber.Ctx ) ( err error ) {
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
func ( handlers *Handlers ) DeleteUser( c fiber.Ctx ) ( err error ) {
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
