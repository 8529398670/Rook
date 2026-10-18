// The game's artwork. The crow and the ROOK logo are the originals from the
// old PHP game, traced into SVG paths by scripts/trace-art.sh (they live in
// art-paths.js), so they look like the real deck but stay crisp at any size.
//
// Everything is built with Dom.svg (createElementNS + setAttribute). Nothing
// here ever goes near innerHTML.
const Art = {
  // Each color's ink, sampled from the original card scans. "text" is the
  // same ink, darkened only where it has to read as small type.
  palette: {
    red:    { main: "#e11922" , text: "#d0141d" },
    black:  { main: "#232425" , text: "#232425" },
    yellow: { main: "#f2b31f" , text: "#e5a50f" },
    green:  { main: "#01a57f" , text: "#009273" },
    rook:   { main: "#2a9eb8" , text: "#1f8ea8" },
  },
  ink: "#1b1d22",
  paper: "#ffffff",
  paperEdge: "#353135",
  cream: "#f3ecd3",

  // The original deck marks colors with ink alone, no suit symbols, so a
  // color in the UI is a tiny blank card printed in that ink. Centred on
  // 0,0; size is the half-height.
  glyph( color , size , fill ) {
    const s = size;
    const paint = fill || ( Art.palette[ color ] || Art.palette.rook ).main;
    return Dom.svg( "rect" , { x: -s * 0.72 , y: -s , width: s * 1.44 , height: s * 2 , rx: s * 0.28 , fill: paint } );
  },

  // A standalone color chip, e.g. inside a trump button or a score chip.
  glyphIcon( color , className ) {
    return Dom.svg( "svg" , { viewBox: "-12 -12 24 24" , class: className || "glyph" , "aria-hidden": "true" } , [
      Art.glyph( color , 10 ),
    ] );
  },

  // One traced picture from art-paths.js as a group in its own pixel space
  // (0,0 to width,height). tones maps each layer name to its fill.
  traced( name , tones ) {
    const picture = ArtPaths[ name ];
    const flip = "translate(0 " + picture.height + ") scale(" + picture.scale + " " + -picture.scale + ")";
    return Dom.svg( "g" , { transform: flip } , Object.keys( tones ).map( function ( layer ) {
      return Dom.svg( "path" , { d: picture[ layer ] , fill: tones[ layer ] } );
    } ) );
  },

  // The crow from the original box art: beak open mid-caw, a fan of cards
  // in one claw, perched on a branch. 706x898.
  bird() {
    return Dom.svg( "g" , { class: "bird" } , [
      Art.traced( "crow" , { cream: Art.cream , light: "#a9aa9c" , mid: "#5b5c52" , dark: "#15150f" } ),
    ] );
  },

  // A complete, sized <svg> with the crow in it.
  birdSvg( className ) {
    const picture = ArtPaths.crow;
    return Dom.svg( "svg" , { viewBox: "0 0 " + picture.width + " " + picture.height , class: className || "bird-art" , "aria-hidden": "true" } , [ Art.bird() ] );
  },

  // The original hand-lettered ROOK logo: cream letters in a black outline
  // with a grey halo. Returned as a group centred on x=0 with its foot at
  // y=0 and width units wide, so callers can place it. tones overrides the
  // layer colors, e.g. a white halo on the teal card back.
  wordmarkGroup( width , tones ) {
    const picture = ArtPaths.logo;
    const scale = ( width || 380 ) / picture.width;
    return Dom.svg( "g" , { class: "wordmark" , transform: "scale(" + scale + ") translate(" + -picture.width / 2 + " " + -picture.height + ")" } , [
      Art.traced( "logo" , Object.assign( {} , Art.logoTones , tones ) ),
    ] );
  },

  logoTones: { halo: "#6e6e6f" , ink: "#0b0b0b" , cream: "#fdf6dc" },

  wordmarkSvg( className ) {
    const picture = ArtPaths.logo;
    return Dom.svg( "svg" , { viewBox: "0 0 " + picture.width + " " + picture.height , class: className || "wordmark-art" , role: "img" , "aria-label": I18n.get( "app.title" ) } , [
      Art.traced( "logo" , Art.logoTones ),
    ] );
  },

  // The avatar colors a player can pick, as hues. Names, not values, are
  // stored, so these can be retuned without touching anyone's account.
  avatarHues: { teal: 188 , coral: 12 , plum: 285 , gold: 40 , forest: 140 , sky: 208 , rose: 340 , berry: 318 },

  // Draw a person's avatar into element: their chosen icon, or their
  // initials, on their chosen color (or one picked from their name).
  avatar( element , person ) {
    const who = person || {};
    Dom.text( element , who.icon || Art.initials( who.name || who.display_name ) );
    element.classList.toggle( "has-icon" , Boolean( who.icon ) );
    const hue = Object.prototype.hasOwnProperty.call( Art.avatarHues , who.color ) ? Art.avatarHues[ who.color ] : Art.hue( who.name || who.display_name );
    Dom.vars( element , { hue: hue } );
  },

  // Initials for a seat badge, e.g. "Ana Brook" -> "AB".
  initials( name ) {
    const words = String( name || "?" ).trim().split( /\s+/ ).filter( Boolean );
    if ( words.length === 0 ) return "?";
    const first = Array.from( words[ 0 ] )[ 0 ] || "?";
    const second = words.length > 1 ? ( Array.from( words[ words.length - 1 ] )[ 0 ] || "" ) : "";
    return ( first + second ).toUpperCase();
  },

  // A stable avatar hue per name, so each person keeps their color.
  hue( name ) {
    let hash = 0;
    Array.from( String( name || "" ) ).forEach( function ( character ) {
      hash = ( hash * 31 + character.codePointAt( 0 ) ) % 360;
    } );
    return hash;
  },
};
