package config

import (
	fmt "fmt"
	os "os"
	filepath "path/filepath"
)

// ForTest is the configuration a test in another package runs on: the
// defaults, with everything that lives on disk inside dir and a fresh key.
// Nothing is looked up or written outside dir. It reads config/defaults.yaml
// itself when main.go has not registered the embedded copy, which is the case
// under `go test`; every package that calls it sits two levels below the
// project root (server/<package>).
func ForTest( dir string ) ( cfg *Config , err error ) {
	if len( defaultsYAML ) == 0 {
		defaults , read_err := os.ReadFile( filepath.Join( ".." , ".." , ConfigDirName , "defaults.yaml" ) )
		if read_err != nil {
			err = fmt.Errorf( "ForTest: %w" , read_err )
			return
		}
		UseEmbedded( defaults , nil )
	}
	file , err := Defaults()
	if err != nil { return }
	file.Server.AppDir = dir
	file.Server.ControlSocket = false
	file.Logs.Enabled = false
	config_path := filepath.Join( dir , ConfigFileName )
	cfg = build( file , config_path , ResolvePaths( file , config_path ) , GenerateSecret() )
	return
}
