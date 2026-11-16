package models

import (
	bytes "bytes"
	binary "encoding/binary"
	crc32 "hash/crc32"
	errors "errors"
	image "image"
	color "image/color"
	gif "image/gif"
	jpeg "image/jpeg"
	png "image/png"
	testing "testing"

	encryption "rook/server/encryption"
)

func testPicture( t *testing.T , width int , height int , format string ) ( result []byte ) {
	t.Helper()
	picture := image.NewRGBA( image.Rect( 0 , 0 , width , height ) )
	for x := 0 ; x < width ; x += 7 {
		picture.Set( x , height / 2 , color.RGBA{ 200 , 40 , 40 , 255 } )
	}
	var out bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode( &out , picture )
	case "jpeg":
		err = jpeg.Encode( &out , picture , nil )
	case "gif":
		err = gif.Encode( &out , picture , nil )
	}
	if err != nil { t.Fatal( err ) }
	result = out.Bytes()
	return
}

// A JPEG with an EXIF block (APP1) spliced in after its start marker.
func withEXIF( raw []byte ) ( result []byte ) {
	segment := []byte{ 0xff , 0xe1 , 0x00 , 0x10 , 'E' , 'x' , 'i' , 'f' , 0 , 0 , 'G' , 'P' , 'S' , 'G' , 'P' , 'S' , 'G' , 'P' }
	result = append( append( append( []byte{} , raw[ :2 ]... ) , segment... ) , raw[ 2: ]... )
	return
}

func TestShrinkBackground( t *testing.T ) {
	out , width , height , err := ShrinkBackground( testPicture( t , 4000 , 2000 , "png" ) )
	if err != nil { t.Fatal( err ) }
	if width != 2560 || height != 1280 {
		t.Fatalf( "4000x2000 shrank to %dx%d, want 2560x1280" , width , height )
	}
	config , format , err := image.DecodeConfig( bytes.NewReader( out ) )
	if err != nil || format != "jpeg" || config.Width != 2560 || config.Height != 1280 {
		t.Fatalf( "stored as %s %dx%d (%v)" , format , config.Width , config.Height , err )
	}

	// Portrait shrinks by its height; small ones keep their size.
	if _ , width , height , err = ShrinkBackground( testPicture( t , 1500 , 3000 , "jpeg" ) ); err != nil || width != 1280 || height != 2560 {
		t.Fatalf( "1500x3000: %dx%d %v" , width , height , err )
	}
	if _ , width , height , err = ShrinkBackground( testPicture( t , 800 , 600 , "png" ) ); err != nil || width != 800 || height != 600 {
		t.Fatalf( "800x600: %dx%d %v" , width , height , err )
	}

	// EXIF does not survive.
	tagged := withEXIF( testPicture( t , 300 , 200 , "jpeg" ) )
	if bytes.Contains( tagged , []byte( "GPSGPS" ) ) == false { t.Fatal( "the test picture has no EXIF" ) }
	if out , _ , _ , err = ShrinkBackground( tagged ); err != nil { t.Fatal( err ) }
	if bytes.Contains( out , []byte( "Exif" ) ) || bytes.Contains( out , []byte( "GPSGPS" ) ) { t.Fatal( "EXIF kept" ) }

	// Not a JPEG, PNG or WebP.
	for name , raw := range map[string][]byte{ "gif": testPicture( t , 20 , 20 , "gif" ) , "junk": []byte( "not a picture at all" ) , "empty": nil } {
		if _ , _ , _ , err = ShrinkBackground( raw ); errors.Is( err , ErrBackgroundFormat ) == false {
			t.Errorf( "%s: %v, want %v" , name , err , ErrBackgroundFormat )
		}
	}

	// A header claiming more pixels than allowed is refused before decoding.
	huge := testPicture( t , 16 , 16 , "png" )
	// PNG IHDR width and height are bytes 16-23.
	copy( huge[ 16:24 ] , []byte{ 0 , 0 , 0x4e , 0x20 , 0 , 0 , 0x4e , 0x20 } )
	binary.BigEndian.PutUint32( huge[ 29:33 ] , crc32.ChecksumIEEE( huge[ 12:29 ] ) )
	if _ , _ , _ , err = ShrinkBackground( huge ); errors.Is( err , ErrBackgroundSize ) == false {
		t.Fatalf( "20000x20000 header: %v, want %v" , err , ErrBackgroundSize )
	}
}

func TestBackgroundsKeepAndShare( t *testing.T ) {
	store := testStore( t )
	backgrounds := NewBackgrounds( store , t.TempDir() , encryption.GenerateKeyHex() )
	alice , err := CreateUser( store , "Alice" , RoleUser )
	if err != nil { t.Fatal( err ) }
	bob , err := CreateUser( store , "Bob" , RoleUser )
	if err != nil { t.Fatal( err ) }

	own , err := backgrounds.Add( alice.ID , false , testPicture( t , 640 , 360 , "png" ) )
	if err != nil { t.Fatal( err ) }
	shared , err := backgrounds.Add( bob.ID , true , testPicture( t , 640 , 360 , "jpeg" ) )
	if err != nil { t.Fatal( err ) }

	if own.CanUse( alice.ID ) == false || own.CanUse( bob.ID ) { t.Fatal( "an own picture is its owner's alone" ) }
	if shared.CanUse( alice.ID ) == false { t.Fatal( "a shared picture is everyone's" ) }

	got , err := backgrounds.Bytes( own )
	if err != nil { t.Fatal( err ) }
	if _ , format , _ := image.DecodeConfig( bytes.NewReader( got ) ); format != "jpeg" { t.Fatalf( "read back as %q" , format ) }

	usable , err := backgrounds.Usable( alice.ID )
	if err != nil || len( usable ) != 2 || usable[ 0 ].ID != own.ID || usable[ 1 ].ID != shared.ID {
		t.Fatalf( "alice may use %+v (%v)" , usable , err )
	}
	if usable , _ = backgrounds.Usable( bob.ID ); len( usable ) != 1 { t.Fatalf( "bob may use %+v" , usable ) }

	// The limit counts one's own only.
	for count := 1 ; count < MaxBackgroundsPerUser ; count++ {
		if _ , err = backgrounds.Add( alice.ID , false , testPicture( t , 32 , 32 , "png" ) ); err != nil { t.Fatal( err ) }
	}
	if _ , err = backgrounds.Add( alice.ID , false , testPicture( t , 32 , 32 , "png" ) ); errors.Is( err , ErrBackgroundLimit ) == false {
		t.Fatalf( "past the limit: %v" , err )
	}

	// Saving a list keeps one's own and shared pictures, and drops anyone else's.
	at := func( value int ) *int { return &value }
	table := func( id string ) *ThemeTable { return &ThemeTable{ Look: TableLookImage , Image: id , X: at( 30 ) , Y: at( 70 ) , Dim: 20 } }
	list := []Theme{
		{ ID: "a" , Table: table( own.ID ) },
		{ ID: "b" , Table: table( shared.ID ) },
		{ ID: "c" , Table: table( "0123456789abcdef01234567" ) },
	}
	kept , err := backgrounds.KeepUsablePictures( list , alice.ID , false )
	if err != nil { t.Fatal( err ) }
	if kept[ 0 ].Table == nil || kept[ 1 ].Table == nil || kept[ 2 ].Table != nil { t.Fatalf( "alice's list kept %+v %+v %+v" , kept[ 0 ].Table , kept[ 1 ].Table , kept[ 2 ].Table ) }
	if kept , _ = backgrounds.KeepUsablePictures( list , bob.ID , false ); kept[ 0 ].Table != nil { t.Fatal( "bob may not use alice's picture" ) }
	if kept , _ = backgrounds.KeepUsablePictures( list , alice.ID , true ); kept[ 0 ].Table != nil || kept[ 1 ].Table == nil {
		t.Fatal( "the defaults take shared pictures only" )
	}

	// Deleting one takes it off the deleter's own themes.
	if err = SetUserThemes( store , alice.ID , []Theme{ { ID: "a" , Name: "A" , Mode: "light" , Accent: "#112233" , Second: "#445566" , Table: table( own.ID ) } } , 0 , "a" ); err != nil {
		t.Fatal( err )
	}
	if err = backgrounds.Delete( own , alice.ID ); err != nil { t.Fatal( err ) }
	if _ , err = backgrounds.Get( own.ID ); errors.Is( err , ErrBackgroundGone ) == false { t.Fatalf( "still there: %v" , err ) }
	after , _ := GetUser( store , alice.ID )
	if after.Themes[ 0 ].Table != nil { t.Fatalf( "theme still names the deleted picture: %+v" , after.Themes[ 0 ].Table ) }
}

func TestCleanThemeTable( t *testing.T ) {
	at := func( value int ) *int { return &value }
	good := []*ThemeTable{
		nil,
		{ Look: "walnut" },
		{ Look: "image" , Image: "0123456789abcdef01234567" , X: at( 0 ) , Y: at( 100 ) , Dim: 70 },
	}
	for _ , table := range good {
		if _ , err := cleanThemeTable( "t" , table ); err != nil { t.Errorf( "%+v: %v" , table , err ) }
	}
	bad := []*ThemeTable{
		{ Look: "carpet" },
		{ Look: "image" , Image: "../../etc/passwd" },
		{ Look: "image" , Image: "0123456789abcdef01234567" , X: at( 101 ) },
		{ Look: "image" , Image: "0123456789abcdef01234567" , Y: at( -1 ) },
		{ Look: "image" , Image: "0123456789abcdef01234567" , Dim: 71 },
	}
	for _ , table := range bad {
		if _ , err := cleanThemeTable( "t" , table ); err == nil { t.Errorf( "%+v: accepted" , table ) }
	}
	// x and y left out are the middle.
	if cleaned , _ := cleanThemeTable( "t" , &ThemeTable{ Look: "image" , Image: "0123456789abcdef01234567" } ); *cleaned.X != 50 || *cleaned.Y != 50 {
		t.Errorf( "no x and y: %d %d" , *cleaned.X , *cleaned.Y )
	}
	// A picture without look image is dropped; look image without one is the mode's own.
	cleaned , _ := cleanThemeTable( "t" , &ThemeTable{ Look: "red" , Image: "0123456789abcdef01234567" , X: at( 10 ) , Dim: 30 } )
	if cleaned.Image != "" || cleaned.X != nil || cleaned.Dim != 0 { t.Errorf( "red with a picture: %+v" , cleaned ) }
	if cleaned , _ = cleanThemeTable( "t" , &ThemeTable{ Look: "image" } ); cleaned != nil { t.Errorf( "image with no picture: %+v" , cleaned ) }
}
