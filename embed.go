package main

import (
	embed "embed"
	fs "io/fs"

	config "rook/server/config"
)

// The frontend, compiled into the binary.
//
// This is what makes build.sh able to produce one file that runs anywhere:
// no ./static directory to ship alongside it, no language.yaml to forget.
// The embed directives have to live in a package at the module root, because
// go:embed cannot reference paths outside its own package directory -- which
// is the only reason this is here rather than inside server/assets.
//
// "all:" includes files whose names begin with "." or "_", which a normal
// embed pattern skips.
//
//go:embed all:static
var embeddedStaticRoot embed.FS

//go:embed config/language.yaml
var embeddedLanguage []byte

// The defaults every missing config.yaml key falls back to, and the note that
// turns them into config/config.example.yaml. See server/config.
//
//go:embed config/defaults.yaml
var embeddedDefaults []byte

//go:embed config/example_header.yaml
var embeddedExampleHeader []byte

// The names random bots can be given, used when bot_names_file is left at
// its default and the file is not on disk (see routes.LoadBotNames).
//
//go:embed misc/biblical-persons.csv
var embeddedBotNames []byte

// The config package needs the defaults before anything else runs -- every
// subcommand, including `config init`, starts from them.
func init() {
	config.UseEmbedded( embeddedDefaults , embeddedExampleHeader )
}

// embeddedStatic re-roots the embedded tree at ./static, so a request for
// "css/style.css" does not have to know it lives at "static/css/style.css"
// inside the binary. This makes it interchangeable with the os.Root
// filesystem server/assets opens for ./static on disk.
func embeddedStatic() ( fsys fs.FS , err error ) {
	fsys , err = fs.Sub( embeddedStaticRoot , "static" )
	return
}
