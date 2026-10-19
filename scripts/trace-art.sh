#!/usr/bin/env bash
# Rebuild static/js/art-paths.js by tracing the original game artwork in
# old_php_version/Images into SVG paths, so the new app draws the very same
# crow and ROOK logo as the old one, but crisp at any size.
#
# Each picture is split into a few flat tones (cream, greys, black or two
# teals), each tone is traced with potrace, and the layers are stacked back
# up in the browser. Needs ImageMagick 7 (magick), potrace and python3.
#
# Usage: ./scripts/trace-art.sh
set -euo pipefail
cd "$( dirname "${BASH_SOURCE[0]}" )/.."

for tool in magick potrace python3; do
  command -v "$tool" > /dev/null || { echo "Missing $tool" >&2; exit 1; }
done

IMAGES="old_php_version/Images"
WORK="$( mktemp -d )"
trap 'rm -rf "$WORK"' EXIT

trace() {
  potrace -s -t "$2" -a 1.1 -O 0.6 -u "$3" "$WORK/$1.pbm" -o "$WORK/$1.svg"
}

# The crow from the box art, holding a fan of cards on a branch. Background
# is found by flood-filling from the corners, so the cream highlights inside
# the bird survive. The stray brush mark above its head is painted out.
magick "$IMAGES/bird.jpg" -alpha set -fuzz 6% -fill none \
  -draw "color 0,0 floodfill" -draw "color 352,0 floodfill" \
  -draw "color 0,448 floodfill" -draw "color 352,448 floodfill" "$WORK/crow.png"
magick "$WORK/crow.png" -alpha extract -negate -resize 200% -blur 0x1 -threshold 50% \
  -fill white -draw "rectangle 0,0 150,90" "$WORK/crow_cream.pbm"
for tone in light:80% mid:50% dark:22%; do
  magick "$IMAGES/bird.jpg" -fill white -draw "rectangle 0,0 75,45" -colorspace gray \
    -resize 200% -blur 0x1 -threshold "${tone#*:}" "$WORK/crow_${tone%%:*}.pbm"
done
for layer in cream light mid dark; do trace "crow_$layer" 6 2; done

# The ROOK logo: a grey halo, a black outline, and cream letters. Only the
# letters are taken from the scan; the scan's own outline is ragged (the K
# most of all), so the outline and halo are grown from the cleaned letters
# instead -- an even, tight band that joins where letters touch -- and their
# insides are filled, so the counters of the R and Os stay solid black.
magick "$IMAGES/rooktext.jpg" -alpha set -fuzz 5% -fill none \
  -draw "color 0,0 floodfill" -draw "color 387,0 floodfill" \
  -draw "color 0,214 floodfill" -draw "color 387,214 floodfill" "$WORK/logo.png"
# Cream is whatever is light and inside the scan's outline, smoothed.
magick "$IMAGES/rooktext.jpg" -colorspace gray -resize 200% -blur 0x1 -threshold 75% \
  \( "$WORK/logo.png" -alpha extract -resize 200% -blur 0x1 -threshold 50% \) \
  -compose multiply -composite \
  -morphology Open Disk:3 -morphology Close Disk:5 -blur 0x2 -threshold 50% "$WORK/logo_letters.png"
fill_holes() {
  magick "$1" -bordercolor black -border 2 -fill red -draw "color 0,0 floodfill" -shave 2 \
    -fill white +opaque red -fill black -opaque red "$2"
}
magick "$WORK/logo_letters.png" -morphology Dilate Disk:15 -morphology Close Disk:10 "$WORK/logo_outline.png"
fill_holes "$WORK/logo_outline.png" "$WORK/logo_ink.png"
magick "$WORK/logo_outline.png" -morphology Dilate Disk:5 "$WORK/logo_grown.png"
fill_holes "$WORK/logo_grown.png" "$WORK/logo_halo.png"
for layer in halo ink letters; do magick "$WORK/logo_$layer.png" -negate "$WORK/logo_$layer.pbm"; done
mv "$WORK/logo_letters.pbm" "$WORK/logo_cream.pbm"
for layer in halo ink cream; do
  potrace -s -t 6 -a 1.2 -O 0.6 -u 2 "$WORK/logo_$layer.pbm" -o "$WORK/logo_$layer.svg"
done

# The face of the Rook card: the teal crow perched on the 20, ROOK down two
# corners and the little crows, traced as one ink layer from everything
# inside the border (124x198 from 6,6), plus the light flecks of the
# engraving on the big crow. The source is small, so it is enlarged 8x.
magick "$IMAGES/cards/rook.PNG" -background white -flatten -crop 124x198+6+6 +repage -colorspace gray "$WORK/rook.png"
magick "$WORK/rook.png" -filter Lanczos -resize 800% -blur 0x2.5 -threshold 72% "$WORK/rookface_ink.pbm"
magick "$WORK/rook.png" -filter Lanczos -resize 800% -blur 0x1 -threshold 66% \
  \( "$WORK/rookface_ink.pbm" -negate -morphology Erode Disk:8 \) -compose multiply -composite \
  -fill black -draw "rectangle 0,720 992,1584" -draw "rectangle 0,0 170,1584" -draw "rectangle 820,0 992,1584" \
  -negate "$WORK/rookface_light.pbm"
potrace -s -t 20 -a 1.1 -O 0.5 -u 1 "$WORK/rookface_ink.pbm" -o "$WORK/rookface_ink.svg"
potrace -s -t 10 -a 1 -O 0.4 -u 1 "$WORK/rookface_light.pbm" -o "$WORK/rookface_light.svg"

# The card back's crow (80x76 from 22,30): black ink only, the teal of the
# back shows through it. A scrap of the logo below its feet is painted out.
magick "$IMAGES/cards/CardBack.PNG" -background white -flatten -crop 80x76+22+30 +repage -colorspace gray \
  -filter Lanczos -resize 800% -blur 0x1.5 -threshold 38% -fill white -draw "rectangle 440,585 640,608" \
  "$WORK/backcrow_ink.pbm"
trace backcrow_ink 12 1

# The card back's logo (112x56 from 14,102): a black outline, a gold ring
# and white letters. Each layer is the union of itself and everything inside
# it, so they nest cleanly. The crow's feet above it and the (R) are painted
# out.
magick "$IMAGES/cards/CardBack.PNG" -background white -flatten -crop 112x56+14+102 +repage \
  -filter Lanczos -resize 800% -blur 0x1.5 "$WORK/backlogo.png"
magick "$WORK/backlogo.png" -colorspace gray -threshold 80% -negate "$WORK/backlogo_white.pbm"
magick "$WORK/backlogo.png" -channel R -separate -threshold 55% -negate "$WORK/backlogo_gold.pbm"
magick "$WORK/backlogo.png" \( -clone 0 -channel R -separate -threshold 55% \) \
  \( -clone 0 -colorspace gray -threshold 30% -negate \) -delete 0 -compose lighten -composite \
  -morphology Close Disk:3 -negate -fill white -draw "rectangle 150,0 420,55" -draw "rectangle 830,0 896,90" \
  "$WORK/backlogo_ink.pbm"
for layer in ink gold white; do trace "backlogo_$layer" 20 1; done

python3 - "$WORK" > static/js/art-paths.js <<'PYTHON'
import re
import sys

work = sys.argv[ 1 ]

def layer( name ):
    source = open( work + "/" + name + ".svg" ).read()
    paths = re.findall( r'<path d="([^"]+)"' , source )
    return re.sub( r"\s+" , " " , " ".join( paths ) ).strip()

def picture( name , layers ):
    source = open( work + "/" + name + "_" + layers[ 0 ] + ".svg" ).read()
    box = re.search( r'viewBox="0 0 ([\d.]+) ([\d.]+)"' , source )
    scale = re.search( r"scale\(([\d.]+),-" , source )
    width , height = round( float( box.group( 1 ) ) ) , round( float( box.group( 2 ) ) )
    lines = [ "  %s: {" % name ]
    lines.append( "    width: %d, height: %d, scale: %s," % ( width , height , float( scale.group( 1 ) ) ) )
    for tone in layers:
        lines.append( '    %s: "%s",' % ( tone , layer( name + "_" + tone ) ) )
    lines.append( "  }," )
    return "\n".join( lines )

print( "// Generated by scripts/trace-art.sh from the original artwork in" )
print( "// old_php_version/Images. Do not edit by hand: re-run the script." )
print( "//" )
print( "// Each picture is a stack of flat tone layers, back to front. Path" )
print( "// coordinates are potrace's: y grows upward and every unit is 1/scale" )
print( "// of a pixel, so draw them under translate(0 height) scale(s, -s)." )
print( "const ArtPaths = {" )
print( picture( "crow" , [ "cream" , "light" , "mid" , "dark" ] ) )
print( picture( "logo" , [ "halo" , "ink" , "cream" ] ) )
print( picture( "rookface" , [ "ink" , "light" ] ) )
print( picture( "backcrow" , [ "ink" ] ) )
print( picture( "backlogo" , [ "ink" , "gold" , "white" ] ) )
print( "};" )
PYTHON

echo "Wrote static/js/art-paths.js ($( wc -c < static/js/art-paths.js | tr -d ' ' ) bytes)"

# The browser tab's icon is drawn from the same art, ahead of time.
python3 scripts/make-favicon.py
