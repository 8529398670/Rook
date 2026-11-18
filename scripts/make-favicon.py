#!/usr/bin/env python3
# Rebuild static/favicon.svg from the traced card art in static/js/art-paths.js,
# so the browser tab shows the same card back the table does (cards.js
# backSymbol): white stock, the teal ring ornament and the ROOK logo, with the
# site's own crow (the one in the lobby header, art.js bird()) in the middle.
#
# It also writes the home-screen icons: static/apple-touch-icon.png (iPhone and
# iPad, which ignore SVG icons) and static/icon-192.png / icon-512.png (the
# web manifest, for Android). Every one is just the card, upright at full
# height, with transparent sides. They need rsvg-convert (brew install librsvg).
#
# The favicon is a static file -- browsers fetch it before any script runs --
# so it cannot draw itself with cards.js; this script does the same drawing
# once, ahead of time. Re-run it after scripts/trace-art.sh.
#
# Usage: ./scripts/make-favicon.py [output]
import re
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path( __file__ ).resolve().parent.parent
SOURCE = ROOT / "static" / "js" / "art-paths.js"

# Kept in step with cards.js and art.js.
TEAL , ORNAMENT , EDGE = "#40aab8" , "#56bac6" , "#2b2b2e"
SCAN_SCALE = 350 / 210


def load_pictures():
    text = SOURCE.read_text()
    pictures = {}
    for match in re.finditer( r"^  (\w+): \{\n(.*?)^  \},?$" , text , re.S | re.M ):
        name , body = match.group( 1 ) , match.group( 2 )
        size = re.search( r"width: (\d+), height: (\d+), scale: ([\d.]+)" , body )
        layers = dict( re.findall( r"^    (\w+): \"([^\"]*)\"" , body , re.M ) )
        pictures[ name ] = { "width": int( size.group( 1 ) ) , "height": int( size.group( 2 ) ) , "scale": float( size.group( 3 ) ) , "layers": layers }
    return pictures


def traced( picture , tones ):
    flip = f"translate(0 {picture['height']}) scale({picture['scale']} {-picture['scale']})"
    paths = "".join( f'<path d="{picture["layers"][layer]}" fill="{fill}"/>' for layer , fill in tones.items() )
    return f'<g transform="{flip}">{paths}</g>'


# cards.js scan(): place a piece where it was cut from the 136x210 scan.
def scan( picture , tones , left , top ):
    x = ( 250 - 136 * SCAN_SCALE ) / 2 + left * SCAN_SCALE
    return f'<g transform="translate({x:.3f} {top * SCAN_SCALE:.3f}) scale({SCAN_SCALE / 8:.5f})">{traced( picture , tones )}</g>'


# Loose specks the tracing left off the bird, in the crow's 706x898 space: a
# dot under the branch and a fleck past the tail. Too small to see in the
# header, they read as stray marks on the icon.
CROW_SPECKS = ( ( 425 , 835 , 450 , 865 ) , ( 670 , 735 , 690 , 755 ) )
# Scan dirt above the letters of the card-back logo (896x448 space): small ink
# bits the trace kept between the crow and the word ROOK.
LOGO_SPECKS = ( ( 410 , 0 , 500 , 40 ) , )


def subpath_box( d , picture ):
    # The traced paths are an absolute M then relative c/l, in the flipped
    # space traced() undoes; return the box in picture pixels.
    x = y = 0.0
    xs , ys = [] , []
    command , numbers = None , []
    for token in re.findall( r"[A-Za-z]|-?\d+\.?\d*" , d ):
        if token.isalpha():
            command , numbers = token , []
            continue
        numbers.append( float( token ) )
        if command == "M" and len( numbers ) == 2:
            x , y = numbers
            command , numbers = "l" , []
        elif command == "l" and len( numbers ) == 2:
            x , y = x + numbers[ 0 ] , y + numbers[ 1 ]
            numbers = []
        elif command == "c" and len( numbers ) == 6:
            x , y = x + numbers[ 4 ] , y + numbers[ 5 ]
            numbers = []
        else:
            continue
        xs.append( x )
        ys.append( y )
    scale , height = picture[ "scale" ] , picture[ "height" ]
    return min( xs ) * scale , height - max( ys ) * scale , max( xs ) * scale , height - min( ys ) * scale


# picture with every sub-shape that lies wholly inside one of specks taken out.
def without_specks( picture , specks ):
    def inside( box ):
        return any( box[ 0 ] >= a and box[ 1 ] >= b and box[ 2 ] <= c and box[ 3 ] <= d for a , b , c , d in specks )
    cleaned = dict( picture , layers={} )
    for name , d in picture[ "layers" ].items():
        parts = re.findall( r"M[^M]*" , d )
        cleaned[ "layers" ][ name ] = "".join( part for part in parts if inside( subpath_box( part , picture ) ) is False )
    return cleaned


# The lobby header's crow, exactly as art.js bird() draws it (same layers and
# tones, less the loose specks), sized to sit clear above the logo in the
# middle of the card.
def crow( picture ):
    height = 116
    scale = height / picture[ "height" ]
    x = ( 250 - picture[ "width" ] * scale ) / 2
    tones = { "cream": "#f3ecd3" , "light": "#a9aa9c" , "mid": "#5b5c52" , "dark": "#15150f" }
    return f'<g transform="translate({x:.3f} 40) scale({scale:.5f})">{traced( without_specks( picture , CROW_SPECKS ) , tones )}</g>'


def back( pictures ):
    rings = "".join(
        f'<circle cx="{x}" cy="{y}" r="48" fill="none" stroke="{ORNAMENT}" stroke-width="4"/>'
        for x , y in ( ( 0 , 0 ) , ( 96 , 0 ) , ( 0 , 96 ) , ( 96 , 96 ) , ( 48 , 48 ) )
    )
    return (
        f'<defs><pattern id="ring" width="96" height="96" patternUnits="userSpaceOnUse" x="29" y="31">'
        f'<rect width="96" height="96" fill="{TEAL}"/>{rings}</pattern></defs>'
        f'<rect x="1.5" y="1.5" width="247" height="347" rx="16" fill="#ffffff" stroke="{EDGE}" stroke-width="3"/>'
        f'<rect x="17" y="17" width="216" height="316" rx="12" fill="url(#ring)"/>'
        + crow( pictures[ "crow" ] )
        + scan( without_specks( pictures[ "backlogo" ] , LOGO_SPECKS ) , { "ink": "#111111" , "gold": "#f5b70f" , "white": "#ffffff" } , 14 , 102 )
    )


HEADER = '  <!-- Generated by scripts/make-favicon.py from the card art in js/art-paths.js. -->\n'


def icon( card ):
    # Just the card: the 250x350 card at the full height of a square, centred,
    # with nothing behind it (the sides are transparent).
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 350 350">\n' + HEADER +
        f'  <g transform="translate(50 0)">{card}</g>\n'
        '</svg>\n'
    )


def main():
    output = Path( sys.argv[ 1 ] ) if len( sys.argv ) > 1 else ROOT / "static" / "favicon.svg"
    card = back( load_pictures() )
    svg = icon( card )
    output.write_text( svg )
    print( f"wrote {output} ({len( svg ) // 1024} KB)" )

    converter = shutil.which( "rsvg-convert" )
    if converter is None:
        sys.exit( "rsvg-convert not found (brew install librsvg): the home-screen PNGs were not rebuilt" )
    for name , pixels in ( ( "apple-touch-icon.png" , 180 ) , ( "icon-192.png" , 192 ) , ( "icon-512.png" , 512 ) ):
        target = output.parent / name
        subprocess.run( [ converter , "-w" , str( pixels ) , "-h" , str( pixels ) , "-o" , str( target ) , str( output ) ] , check=True )
        print( f"wrote {target}" )


main()
