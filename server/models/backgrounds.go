package models

import (
	bytes "bytes"
	hex "encoding/hex"
	errors "errors"
	fmt "fmt"
	image "image"
	jpeg "image/jpeg"
	_ "image/png"
	os "os"
	filepath "path/filepath"
	regexp "regexp"
	sort "sort"
	time "time"

	draw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	db "rook/server/db"
	encryption "rook/server/encryption"
)

// Table pictures: photos people upload to lie on the game table instead of
// one of the drawn looks (see ThemeTable). A picture is someone's own, used
// in their own themes, or shared -- uploaded by an admin for the default
// themes everyone follows -- which any signed-in person may see and use.
//
// Every picture is decoded and drawn again as a JPEG no larger than
// BackgroundMaxSide on its long side, whatever came in: that strips EXIF
// (a phone photo's GPS position among it) and anything in the file that is
// not the picture. The browser already shrinks a photo before sending it
// (static/js/backgrounds.js), so it fits the request size limit; the server
// shrinks again for a caller that did not.
//
// The bytes live encrypted in <storage>/backgrounds/<id>, under the "files"
// subkey of secret.key; the record (who, size) is in the backgrounds bucket.
const (
	// BackgroundMaxSide is the long side a stored picture is shrunk to.
	BackgroundMaxSide = 2560
	// BackgroundMaxInputSide and BackgroundMaxPixels refuse a picture before
	// it is decoded, so a tiny file that claims to be enormous cannot take
	// the server's memory.
	BackgroundMaxInputSide = 10000
	BackgroundMaxPixels    = 50_000_000
	// The most pictures one person keeps, and the most shared ones.
	MaxBackgroundsPerUser = 6
	MaxBackgroundsShared  = 12
	backgroundQuality     = 85
	backgroundDirName     = "backgrounds"
)

var (
	ErrBackgroundFormat = errors.New( "background_format" )
	ErrBackgroundSize   = errors.New( "background_too_big" )
	ErrBackgroundLimit  = errors.New( "background_limit" )
	ErrBackgroundGone   = errors.New( "background_not_found" )
	backgroundID        = regexp.MustCompile( `^[0-9a-f]{24}$` )
)

// Background is one picture's record, as stored. The API answers
// BackgroundRow, which says "mine" instead of whose it is.
type Background struct {
	ID        string    `json:"id"`
	OwnerID   uint64    `json:"owner_id"`
	Shared    bool      `json:"shared"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int       `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// BackgroundRow is a picture as the API shows it to one person.
type BackgroundRow struct {
	ID        string    `json:"id"`
	Shared    bool      `json:"shared" jsonschema_description:"Uploaded by an admin for the default themes: everyone may use it."`
	Mine      bool      `json:"mine" jsonschema_description:"You uploaded it, so you may delete it."`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int       `json:"size" jsonschema_description:"Bytes."`
	CreatedAt time.Time `json:"created_at"`
}

func ( BackgroundRow ) APIName() ( name string ) { name = "Background"; return }

func ( BackgroundRow ) APIDoc() ( text string ) {
	text = "A table picture. Its bytes are at GET /api/backgrounds/{id}, a JPEG at most 2560 pixels on its long side."
	return
}

// Row is the picture as this person sees it.
func ( background Background ) Row( user_id uint64 ) ( result BackgroundRow ) {
	result = BackgroundRow{ ID: background.ID , Shared: background.Shared , Mine: background.OwnerID == user_id ,
		Width: background.Width , Height: background.Height , Size: background.Size , CreatedAt: background.CreatedAt }
	return
}

// CanUse says whether this person may see and use the picture.
func ( background Background ) CanUse( user_id uint64 ) ( result bool ) {
	result = background.Shared || background.OwnerID == user_id
	return
}

func ValidBackgroundID( id string ) ( result bool ) {
	result = backgroundID.MatchString( id )
	return
}

// ShrinkBackground decodes a JPEG, PNG or WebP and answers it as a JPEG no
// larger than BackgroundMaxSide on its long side, with its new size.
func ShrinkBackground( raw []byte ) ( result []byte , width int , height int , err error ) {
	config , format , config_err := image.DecodeConfig( bytes.NewReader( raw ) )
	if config_err != nil || ( format != "jpeg" && format != "png" && format != "webp" ) {
		err = ErrBackgroundFormat
		return
	}
	if config.Width < 1 || config.Height < 1 || config.Width > BackgroundMaxInputSide || config.Height > BackgroundMaxInputSide ||
		config.Width * config.Height > BackgroundMaxPixels {
		err = ErrBackgroundSize
		return
	}
	picture , _ , decode_err := image.Decode( bytes.NewReader( raw ) )
	if decode_err != nil {
		err = ErrBackgroundFormat
		return
	}
	bounds := picture.Bounds()
	width , height = bounds.Dx() , bounds.Dy()
	if width > BackgroundMaxSide || height > BackgroundMaxSide {
		if width >= height {
			height = max( 1 , height * BackgroundMaxSide / width )
			width = BackgroundMaxSide
		} else {
			width = max( 1 , width * BackgroundMaxSide / height )
			height = BackgroundMaxSide
		}
	}
	// Drawn onto an opaque canvas either way: a PNG's transparency becomes
	// black rather than whatever JPEG would make of it.
	canvas := image.NewRGBA( image.Rect( 0 , 0 , width , height ) )
	draw.CatmullRom.Scale( canvas , canvas.Bounds() , picture , bounds , draw.Src , nil )
	var out bytes.Buffer
	if err = jpeg.Encode( &out , canvas , &jpeg.Options{ Quality: backgroundQuality } ); err != nil { return }
	result = out.Bytes()
	return
}

// Backgrounds keeps the pictures: the records in the store, the bytes in dir.
type Backgrounds struct {
	Store *db.Store
	// Dir is <storage>/backgrounds.
	Dir string
	// Key is the files subkey of secret.key.
	Key string
}

func NewBackgrounds( store *db.Store , storage_dir string , secret_key string ) ( result *Backgrounds ) {
	result = &Backgrounds{
		Store: store,
		Dir:   filepath.Join( storage_dir , backgroundDirName ),
		Key:   encryption.SubKey( secret_key , encryption.PurposeFiles ),
	}
	return
}

// All is every picture, oldest first.
func ( backgrounds *Backgrounds ) All() ( result []Background , err error ) {
	err = backgrounds.Store.ForEach( db.BucketBackgrounds , func() any { return &Background{} } , func( key []byte , item any ) bool {
		result = append( result , *item.( *Background ) )
		return true
	} )
	sort.Slice( result , func( a , b int ) bool { return result[ a ].CreatedAt.Before( result[ b ].CreatedAt ) } )
	return
}

// Usable is what this person may pick: their own, then the shared ones.
func ( backgrounds *Backgrounds ) Usable( user_id uint64 ) ( result []Background , err error ) {
	all , err := backgrounds.All()
	if err != nil { return }
	result = []Background{}
	for _ , background := range all {
		if background.Shared == false && background.OwnerID == user_id { result = append( result , background ) }
	}
	for _ , background := range all {
		if background.Shared { result = append( result , background ) }
	}
	return
}

func ( backgrounds *Backgrounds ) Get( id string ) ( result Background , err error ) {
	if ValidBackgroundID( id ) == false {
		err = ErrBackgroundGone
		return
	}
	err = backgrounds.Store.Get( db.BucketBackgrounds , []byte( id ) , &result )
	if errors.Is( err , db.ErrNotFound ) { err = ErrBackgroundGone }
	return
}

// Add shrinks a picture and keeps it: the uploader's own, or shared.
func ( backgrounds *Backgrounds ) Add( owner_id uint64 , shared bool , raw []byte ) ( result Background , err error ) {
	all , err := backgrounds.All()
	if err != nil { return }
	count := 0
	for _ , background := range all {
		if ( shared && background.Shared ) || ( shared == false && background.Shared == false && background.OwnerID == owner_id ) { count++ }
	}
	limit := MaxBackgroundsPerUser
	if shared { limit = MaxBackgroundsShared }
	if count >= limit {
		err = ErrBackgroundLimit
		return
	}
	picture , width , height , err := ShrinkBackground( raw )
	if err != nil { return }
	sealed , err := encryption.ChaChaEncryptBytes( backgrounds.Key , picture )
	if err != nil { return }
	if err = os.MkdirAll( backgrounds.Dir , 0o700 ); err != nil { return }
	result = Background{
		ID:        hex.EncodeToString( encryption.GenerateRandomBytes( 12 ) ),
		OwnerID:   owner_id,
		Shared:    shared,
		Width:     width,
		Height:    height,
		Size:      len( picture ),
		CreatedAt: time.Now().UTC(),
	}
	if err = os.WriteFile( filepath.Join( backgrounds.Dir , result.ID ) , sealed , 0o600 ); err != nil { return }
	if err = backgrounds.Store.Put( db.BucketBackgrounds , []byte( result.ID ) , result ); err != nil {
		os.Remove( filepath.Join( backgrounds.Dir , result.ID ) )
	}
	return
}

// Bytes is a picture's JPEG.
func ( backgrounds *Backgrounds ) Bytes( background Background ) ( result []byte , err error ) {
	sealed , err := os.ReadFile( filepath.Join( backgrounds.Dir , background.ID ) )
	if err != nil {
		err = fmt.Errorf( "background %s: %w" , background.ID , err )
		return
	}
	result , err = encryption.ChaChaDecryptBytes( backgrounds.Key , sealed )
	return
}

// Delete removes a picture, its bytes, and the places that point at it that
// the deleter owns: their own themes, and for a shared one, the defaults.
// Anyone else's list that still names it shows the look underneath.
func ( backgrounds *Backgrounds ) Delete( background Background , user_id uint64 ) ( err error ) {
	if err = backgrounds.Store.Delete( db.BucketBackgrounds , []byte( background.ID ) ); err != nil { return }
	if remove_err := os.Remove( filepath.Join( backgrounds.Dir , background.ID ) ); remove_err != nil && errors.Is( remove_err , os.ErrNotExist ) == false {
		err = remove_err
		return
	}
	gone := map[string]bool{ background.ID: true }
	err = backgrounds.Store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			user := item.( *User )
			user.Themes = dropThemePictures( user.Themes , func( id string ) bool { return gone[ id ] == false } )
			return
		} )
	if err != nil || background.Shared == false { return }
	defaults , builtins , err := ThemeDefaults( backgrounds.Store )
	if err != nil || defaults == nil { return }
	err = SetThemeDefaults( backgrounds.Store , dropThemePictures( defaults , func( id string ) bool { return gone[ id ] == false } ) , builtins )
	return
}

// KeepUsablePictures takes out of a list any picture it may not name: one
// that is gone, or someone else's own. defaults allows shared ones only.
// It never refuses the list: a picture deleted since the browser last
// looked just leaves its theme on the look underneath.
func ( backgrounds *Backgrounds ) KeepUsablePictures( themes []Theme , user_id uint64 , defaults bool ) ( result []Theme , err error ) {
	if themes == nil { return }
	all , err := backgrounds.All()
	if err != nil { return }
	allowed := map[string]bool{}
	for _ , background := range all {
		if background.Shared || ( defaults == false && background.OwnerID == user_id ) { allowed[ background.ID ] = true }
	}
	result = dropThemePictures( themes , func( id string ) bool { return allowed[ id ] } )
	return
}

// dropThemePictures answers the list with every picture keep refuses taken
// off its theme, which goes back to the look the mode starts with.
func dropThemePictures( themes []Theme , keep func( id string ) bool ) ( result []Theme ) {
	if themes == nil { return }
	result = make( []Theme , len( themes ) )
	for index , theme := range themes {
		if theme.Table != nil && theme.Table.Image != "" && keep( theme.Table.Image ) == false {
			table := *theme.Table
			table.Image = ""
			if table.Look == TableLookImage { table.Look = "" }
			theme.Table = &table
			if table.Look == "" { theme.Table = nil }
		}
		result[ index ] = theme
	}
	return
}
