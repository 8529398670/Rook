package botnames

import (
	os "os"
	filepath "path/filepath"
	slices "slices"
	testing "testing"
)

func TestLoadSumsAndSorts( t *testing.T ) {
	path := filepath.Join( t.TempDir() , "names.csv" )
	data := "sex,name,total_count\nM,John,50\nF,Mary,70\nM,Jordan,30\nF,Jordan,30\nF,Abby,50\nM,,9\nM,Bad,x\n"
	if err := os.WriteFile( path , []byte( data ) , 0o600 ); err != nil { t.Fatal( err ) }
	names , err := Load( path )
	if err != nil { t.Fatal( err ) }
	if want := []string{ "Mary" , "Jordan" , "Abby" , "John" }; slices.Equal( names , want ) == false {
		t.Fatalf( "got %v want %v" , names , want )
	}
	if top := Top( names , 2 ); slices.Equal( top , []string{ "Mary" , "Jordan" } ) == false { t.Fatalf( "top 2: %v" , top ) }
	if top := Top( names , 0 ); len( top ) != 4 { t.Fatalf( "default top: %v" , top ) }
}

func TestLoadNeedsColumns( t *testing.T ) {
	path := filepath.Join( t.TempDir() , "names.csv" )
	if err := os.WriteFile( path , []byte( "first,count\nA,1\n" ) , 0o600 ); err != nil { t.Fatal( err ) }
	if _ , err := Load( path ); err != ErrNoColumns { t.Fatalf( "got %v" , err ) }
}
