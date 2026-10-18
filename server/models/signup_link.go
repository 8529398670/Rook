package models

import (
	errors "errors"
	sort "sort"
	strings "strings"
	time "time"

	db "rook/server/db"
	encryption "rook/server/encryption"
)

var (
	ErrSignupLinkInvalid = errors.New( "sign-up link is invalid, expired, or revoked" )
	ErrSignupLinkFull    = errors.New( "sign-up link has no sign-ups left" )
	ErrNotLinkAccount    = errors.New( "that account did not sign up through this link" )
)

// SignupLink is a reusable sign-up link for a group: a family, a club. Each
// time someone opens it they type their name. A new name becomes a new
// account, up to MaxUses of them. A name like one that already signed up
// through this link is offered back ("did you mean...?"), which is how the
// same person signs in on their next device.
//
// Picking an existing name signs you in as that person, so anyone holding
// the link can be anyone who joined through it. Two limits keep that sane:
// only accounts this link created can be picked (never an admin, never
// someone who joined another way), and a person can set a PIN in Settings,
// which is then asked for.
//
// Same "<id>.<secret>" shape as the other credentials. The secret is a long
// random token, so sha256 is enough; bcrypt is for low-entropy secrets.
type SignupLink struct {
	ID         string     `json:"id"`
	SecretHash string     `json:"secret_hash"`
	Label      string     `json:"label"`
	MaxUses    int        `json:"max_uses"`
	Accounts   []uint64   `json:"accounts"`
	CreatedBy  uint64     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func ( link *SignupLink ) Remaining() ( result int ) {
	result = link.MaxUses - len( link.Accounts )
	if result < 0 { result = 0 }
	return
}

func ( link *SignupLink ) Live() ( result bool ) {
	result = link.RevokedAt == nil && time.Now().UTC().Before( link.ExpiresAt )
	return
}

func ( link *SignupLink ) Owns( user_id uint64 ) ( result bool ) {
	for _ , account := range link.Accounts {
		if account == user_id {
			result = true
			return
		}
	}
	return
}

func IssueSignupLink( store *db.Store , label string , max_uses int , ttl time.Duration , created_by uint64 ) ( credential string , link *SignupLink , err error ) {
	id := encryption.GenerateURLToken( 12 )
	secret := encryption.GenerateURLToken( 24 )
	now := time.Now().UTC()
	link = &SignupLink{
		ID:         id,
		SecretHash: encryption.Sha256Hex( secret ),
		Label:      strings.TrimSpace( label ),
		MaxUses:    max_uses,
		Accounts:   []uint64{},
		CreatedBy:  created_by,
		CreatedAt:  now,
		ExpiresAt:  now.Add( ttl ),
	}
	if err = store.Put( db.BucketSignupLinks , []byte( id ) , link ); err != nil {
		link = nil
		return
	}
	credential = id + "." + secret
	return
}

// LoadSignupLink checks a credential and returns its live link. Every way it
// can be wrong answers the same, like a login link.
func LoadSignupLink( store *db.Store , credential string ) ( link *SignupLink , err error ) {
	id , secret , ok := splitCredential( credential )
	if ok == false {
		err = ErrSignupLinkInvalid
		return
	}
	link = &SignupLink{}
	if get_err := store.Get( db.BucketSignupLinks , []byte( id ) , link ); get_err != nil {
		link , err = nil , ErrSignupLinkInvalid
		return
	}
	if encryption.ConstantTimeEqual( link.SecretHash , encryption.Sha256Hex( secret ) ) == false || link.Live() == false {
		link , err = nil , ErrSignupLinkInvalid
	}
	return
}

// RecordSignup adds an account to the link inside one transaction, so two
// people signing up at the same moment cannot both take the last place.
func RecordSignup( store *db.Store , link_id string , user_id uint64 ) ( err error ) {
	err = store.UpdateValue( db.BucketSignupLinks , []byte( link_id ) ,
		func() any { return &SignupLink{} } ,
		func( item any ) ( mutate_err error ) {
			link := item.( *SignupLink )
			if link.Remaining() <= 0 {
				mutate_err = ErrSignupLinkFull
				return
			}
			link.Accounts = append( link.Accounts , user_id )
			return
		} )
	return
}

func ListSignupLinks( store *db.Store ) ( links []*SignupLink , err error ) {
	links = []*SignupLink{}
	err = store.ForEach( db.BucketSignupLinks ,
		func() any { return &SignupLink{} } ,
		func( key []byte , item any ) bool {
			links = append( links , item.( *SignupLink ) )
			return true
		} )
	sort.Slice( links , func( i int , j int ) bool { return links[ i ].CreatedAt.After( links[ j ].CreatedAt ) } )
	return
}

func RevokeSignupLink( store *db.Store , link_id string ) ( err error ) {
	err = store.UpdateValue( db.BucketSignupLinks , []byte( link_id ) ,
		func() any { return &SignupLink{} } ,
		func( item any ) ( mutate_err error ) {
			now := time.Now().UTC()
			item.( *SignupLink ).RevokedAt = &now
			return
		} )
	return
}

// SignupMatches are this link's accounts whose names are close to typed,
// closest first. Disabled accounts and admins are never offered.
func SignupMatches( store *db.Store , link *SignupLink , typed string ) ( matches []*User , err error ) {
	type scored struct {
		user  *User
		score int
	}
	found := []scored{}
	for _ , user_id := range link.Accounts {
		user , get_err := GetUser( store , user_id )
		if get_err != nil || user.Disabled() || user.IsAdmin() { continue }
		if score := NameCloseness( typed , user.DisplayName ); score >= 0 {
			found = append( found , scored{ user , score } )
		}
	}
	sort.SliceStable( found , func( i int , j int ) bool { return found[ i ].score < found[ j ].score } )
	matches = []*User{}
	for _ , entry := range found {
		matches = append( matches , entry.user )
	}
	return
}
