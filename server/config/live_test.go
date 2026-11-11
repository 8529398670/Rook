package config

import (
	fmt "fmt"
	os "os"
	filepath "path/filepath"
	strings "strings"
	testing "testing"
)

// newTestHolder is a Holder over a config.yaml (the defaults) and a
// secret.key in a temporary folder.
func newTestHolder( t *testing.T ) ( holder *Holder , path string ) {
	t.Helper()
	useTestDefaults( t )
	dir := t.TempDir()
	path = filepath.Join( dir , "config.yaml" )
	if _ , err := WriteNew( path ); err != nil { t.Fatal( err ) }
	file , err := ReadFile( path )
	if err != nil { t.Fatal( err ) }
	key , err := ReadSecret( SecretPathFor( path ) )
	if err != nil { t.Fatal( err ) }
	cfg := build( file , path , ResolvePaths( file , path ) , key )
	cfg.raw , _ = os.ReadFile( path )
	holder = NewHolder( cfg )
	return
}

func currentVersion( t *testing.T , path string ) ( version string ) {
	raw , err := os.ReadFile( path )
	if err != nil { t.Fatal( err ) }
	version = Version( raw )
	return
}




func TestPollAppliesAHandEdit( t *testing.T ) {
	holder , path := newTestHolder( t )
	notified := 0
	holder.OnChange( func( _ *Config , _ *Config ) { notified++ } )

	raw , _ := os.ReadFile( path )
	edited := strings.Replace( string( raw ) , "rate_limit_max: 120" , "rate_limit_max: 7  " , 1 )
	os.WriteFile( path , []byte( edited ) , 0o600 )
	holder.Poll()
	if holder.Get().RateLimitMax != 120 {
		t.Fatal( "applied on the first sighting -- it should wait for a second" )
	}
	holder.Poll()
	if holder.Get().RateLimitMax != 7 || notified != 1 {
		t.Fatalf( "hand edit not applied: %d, notified %d" , holder.Get().RateLimitMax , notified )
	}

	// An invalid edit keeps the last good config and says why.
	os.WriteFile( path , []byte( strings.Replace( edited , fmt.Sprintf( "port: %d" , holder.Boot().Port ) , "port: 0" , 1 ) ) , 0o600 )
	holder.Poll()
	holder.Poll()
	if holder.Get().RateLimitMax != 7 {
		t.Fatal( "last good config lost" )
	}

	// Fixing it clears the error.
	os.WriteFile( path , []byte( strings.Replace( edited , "rate_limit_max: 7  " , "rate_limit_max: 8  " , 1 ) ) , 0o600 )
	holder.Poll()
	holder.Poll()
	if holder.Get().RateLimitMax != 8 {
		t.Fatalf( "not recovered: %d" , holder.Get().RateLimitMax )
	}

}



func TestWriteInPlaceKeepsTheFileAndABackup( t *testing.T ) {
	dir := t.TempDir()
	path := filepath.Join( dir , "config.yaml" )
	os.WriteFile( path , []byte( "a long first version\n" ) , 0o640 )
	before , _ := os.Stat( path )
	backup := filepath.Join( dir , "data" , BackupFileName )
	os.MkdirAll( filepath.Dir( backup ) , 0o700 )
	if err := writeConfigFile( path , []byte( "short\n" ) , true , backup ); err != nil { t.Fatal( err ) }
	after , _ := os.Stat( path )
	if os.SameFile( before , after ) == false {
		t.Fatal( "in-place write replaced the file" )
	}
	if raw , _ := os.ReadFile( path ); string( raw ) != "short\n" {
		t.Fatalf( "got %q" , raw )
	}
	if raw , _ := os.ReadFile( backup ); string( raw ) != "a long first version\n" {
		t.Fatalf( "backup %q" , raw )
	}

	if err := writeConfigFile( path , []byte( "renamed\n" ) , false , "" ); err != nil { t.Fatal( err ) }
	renamed , _ := os.Stat( path )
	if renamed.Mode().Perm() != 0o640 {
		t.Fatalf( "mode not kept: %o" , renamed.Mode().Perm() )
	}
}

