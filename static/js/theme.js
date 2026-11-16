// How the page looks: light, dark, or another theme.
//
// Loaded in <head>, before any stylesheet, and run immediately: applying the
// theme kept in this browser here -- before first paint -- is what stops the
// page flashing light and then switching.
//
// Light is the default and stays the default. The app never follows the
// operating system's dark setting; dark only happens because someone picked
// it: a dark theme, or Light/Dark on a theme in the editor on the Display
// screen (themes_editor.js) -- the one light/dark switch there is. Dark is <html data-theme="dark">, which style.css reads;
// light is the attribute being absent.
//
// A theme is a mode and a few colors. It can also carry its colors for the
// other mode (a light theme's "dark", a dark theme's "light"), so switching
// it between light and dark swaps them in -- see setMode(). colors() works out
// everything else the page needs from those -- text, links, borders, tints --
// each pushed until it is readable. Its overrides replace any of the
// worked-out colors by name, and dropping one puts the readable color back.
// rookVars() hands them to the custom properties style.css already uses
// (--fg, --bg, --accent, ...).
//
// Default is Rook's own look. While it is in use unchanged nothing is set on
// the page at all, so the stylesheet's own light and dark colors stand.
//
// Which themes there are, and which one someone uses, is kept on their
// account (server/models/themes.go): their own list, or until they change
// something, the defaults an admin set, or until an admin does, the built-in
// ones below. The Display screen (themes_editor.js) edits both lists.
//
// The theme in use is also kept in this browser, whole, so the first paint --
// and the pages before signing in -- are already in it. The account is the
// authority: load( me ) replaces what was kept as soon as /api/me answers.
//
// localStorage is a per-browser convenience, so every access is wrapped: a
// private window or blocked storage just means the default theme.
//
// Brand colors live in ThemePalettes, named, whether a theme uses them or not.
const ThemePalettes = {
  // Wright State University. Primary: green and gold. The rest are its accents.
  wsu: {
    green: "#046a38",
    gold: "#cba052",
    dark_green: "#00482b",
    deep_gold: "#ae8643",
    yellow: "#f8e08e",
    leaf: "#719949",
    beige: "#dcd59a",
    black: "#101820",
    faded_gold: "#e7e3c6",
    off_white: "#f2f1f0",
  },
};

const Theme = {
  // Rook's key from before themes: "light" or "dark" there still reads.
  storageKey: "rook.theme",
  // What redraws when the theme changes: the Appearance screen.
  onChange: [],

  // The built-in themes: what everyone has until an admin saves defaults.
  // accent is the main color: what you can press. second says where you are:
  // the current sidebar item. background and surface are the page and the
  // cards, when the mode's will not do. Names come from language.yaml
  // (theme.preset_<id>) until a list is saved; a saved list keeps its own.
  //
  // Every one has colors for both modes: the ones it starts in, and the other
  // mode's under its name, which the editor's Light/Dark swaps in (setMode).
  // Default comes first and starts light: it is what a new browser shows, and
  // its colors are style.css's own.
  builtIn: [
    { id: "default" , mode: "light" , accent: "#157f92" , second: "#a8641c" ,
      dark: { accent: "#4cc3d6" , second: "#e8b45e" } },
    { id: "ocean" , mode: "light" , accent: "#0b7285" , second: "#bd5418" , background: "#eef6f8" , surface: "#ffffff" ,
      dark: { accent: "#3bc9db" , second: "#ff922b" , background: "#0b1a1f" , surface: "#12262d" } },
    { id: "forest" , mode: "light" , accent: "#28843b" , second: "#8a6a12" , background: "#f1f6ef" , surface: "#ffffff" ,
      dark: { accent: "#69db7c" , second: "#e0b84c" , background: "#0f1a12" , surface: "#17251a" } },
    { id: "sepia" , mode: "light" , accent: "#9c5a1c" , second: "#2f6f6a" , background: "#f6f0e4" , surface: "#fbf7ef" ,
      dark: { accent: "#e0a15e" , second: "#6cc4bb" , background: "#1c1712" , surface: "#262019" } },
    { id: "grape" , mode: "light" , accent: "#7048c8" , second: "#c0267e" , background: "#f5f2fc" , surface: "#ffffff" ,
      dark: { accent: "#b197fc" , second: "#f783ac" , background: "#15111f" , surface: "#1e182c" } },
    { id: "pink" , mode: "light" , accent: "#c2255c" , second: "#6b3fc0" , background: "#fdf0f5" , surface: "#ffffff" ,
      dark: { accent: "#f783ac" , second: "#b197fc" , background: "#1f1018" , surface: "#2a1621" } },
    // Wright State University: Green on Off White; switched to dark, Gold on Black with Dark Green panels.
    { id: "wsu" , mode: "light" , accent: ThemePalettes.wsu.green , second: ThemePalettes.wsu.gold , background: ThemePalettes.wsu.off_white , surface: "#ffffff" ,
      dark: { accent: ThemePalettes.wsu.gold , second: ThemePalettes.wsu.yellow , background: ThemePalettes.wsu.black , surface: ThemePalettes.wsu.dark_green } },
    { id: "midnight" , mode: "dark" , accent: "#8c7cff" , second: "#ffb86b" , background: "#0e1020" , surface: "#171a30" ,
      light: { accent: "#5b4bd6" , second: "#c26a00" , background: "#f3f3fb" , surface: "#ffffff" } },
    { id: "slate" , mode: "dark" , accent: "#4fb3a9" , second: "#e6b450" , background: "#15191e" , surface: "#1d232a" ,
      light: { accent: "#2b7a72" , second: "#a0711a" , background: "#f2f4f6" , surface: "#ffffff" } },
    { id: "cerise" , mode: "dark" , accent: "#f06595" , second: "#ffa8d0" , background: "#1f0d16" , surface: "#2a1220" ,
      light: { accent: "#b0164c" , second: "#e64980" , background: "#fdf2f6" , surface: "#ffffff" } },
  ],

  // What a theme leaves out: Rook's own page, card and text, and the outlines
  // on the card that led the trick and the one that won it, as in style.css.
  modes: {
    light: { background: "#f4eee3" , surface: "#fffdf8" , text: "#1f1d1a" , ledRing: "#111111" , winRing: "#cba052" },
    dark: { background: "#111416" , surface: "#1a1e22" , text: "#ecebe6" , ledRing: "#000000" , winRing: "#cba052" },
  },

  // The button colors besides the main one that Rook draws: danger (delete,
  // leave). Each gets a fill and the label on it (Fg), worked out per theme.
  // The server accepts overrides for every Bootstrap variant; Rook only has
  // this one.
  variants: [ "danger" ],
  // What danger starts from in each mode, before the theme's accent is mixed
  // in (buttonHarmony percent) so it belongs to it.
  semantic: {
    light: { danger: "#c0392b" },
    dark: { danger: "#ef6b5c" },
  },
  buttonHarmony: 12,

  // A theme's button: shape, weight, depth and case. Missing or unknown
  // choices fall back to buttonDefault, which is how Rook's buttons have
  // always looked: rounded, bold, flat, as written.
  buttonDefault: { shape: "rounded" , weight: "bold" , depth: "flat" , case: "normal" },
  buttonChoices: {
    shape: { square: { "--app-btn-radius": "0px" } , soft: { "--app-btn-radius": "6px" } ,
      rounded: { "--app-btn-radius": "12px" } , pill: { "--app-btn-radius": "999px" } },
    weight: { normal: { "--app-btn-weight": "450" } , medium: { "--app-btn-weight": "550" } , bold: { "--app-btn-weight": "650" } },
    depth: {
      flat: { "--app-btn-shadow": "none" , "--app-btn-image": "none" },
      raised: { "--app-btn-shadow": "0 1px 2px rgba(0, 0, 0, 0.2), 0 3px 8px rgba(0, 0, 0, 0.14)" , "--app-btn-image": "none" },
      glossy: { "--app-btn-shadow": "inset 0 1px 0 rgba(255, 255, 255, 0.3), 0 1px 3px rgba(0, 0, 0, 0.25)" ,
        "--app-btn-image": "linear-gradient(180deg, rgba(255, 255, 255, 0.24), rgba(255, 255, 255, 0) 55%, rgba(0, 0, 0, 0.08))" },
    },
    case: { normal: { "--app-btn-transform": "none" , "--app-btn-spacing": "normal" } ,
      upper: { "--app-btn-transform": "uppercase" , "--app-btn-spacing": "0.06em" } },
  },

  // How much of the second color goes into each layer, in percent, so a nested
  // card stands off its card and the card off the page.
  tints: { page: 3.5 , surface: 1.75 , nested: 8.75 , chrome: 10 },

  // The fields of colors() a theme may override (the server checks the same
  // names, server/models/themes.go).
  fields: [ "text" , "accent" , "accentFg" , "second" , "secondFg" , "link" , "navLink" ,
    "background" , "surface" , "nested" , "chrome" , "border" , "ledRing" , "winRing" ],

  // Pairs that must stay readable: [ front , back ] fields of colors(). Names
  // are language.yaml keys (theme.check_<name>).
  pairs: {
    text_card: [ "text" , "surface" ],
    text_nested: [ "text" , "nested" ],
    link: [ "link" , "nested" ],
    button: [ "accentFg" , "accent" ],
    active_nav: [ "secondFg" , "second" ],
    sidebar_link: [ "navLink" , "chrome" ],
  },

  // The themes on offer, and the one in use (its id).
  list: [],
  // Whether list is the person's own rather than the defaults.
  custom: false,
  current: "default",
  // True once /api/me has answered: from then on changes are saved.
  signedIn: false,
  // What this browser had kept, for someone whose account never chose.
  restored: null,
  // WCAG AA for text.
  minContrast: 4.5,

  // A built-in theme with its name filled in, ready to be saved in a list.
  named( theme ) {
    const copy = this.copy( theme );
    if ( !copy.name ) copy.name = ( typeof I18n === "undefined" ? "" : I18n.get( "theme.preset_" + theme.id ) ) || theme.id;
    return copy;
  },

  // A copy holding only what the server keeps (models.Theme): it refuses a
  // list with any other field in it, so nothing else may ride along.
  copy( theme ) {
    const pick = ( from , names ) => {
      const result = {};
      names.forEach( ( name ) => { if ( from[ name ] !== undefined && from[ name ] !== null ) result[ name ] = from[ name ]; } );
      return result;
    };
    const copy = pick( theme , [ "id" , "name" , "mode" , "accent" , "second" , "background" , "surface" ] );
    copy.overrides = Object.assign( {} , theme.overrides || {} );
    if ( theme.button ) copy.button = pick( theme.button , [ "shape" , "weight" , "depth" , "case" ] );
    Object.keys( this.modes ).forEach( ( mode ) => {
      if ( theme[ mode ] ) copy[ mode ] = Object.assign( pick( theme[ mode ] , [ "accent" , "second" , "background" , "surface" ] ) ,
        { overrides: Object.assign( {} , theme[ mode ].overrides || {} ) } );
    } );
    return copy;
  },

  // The fields that belong to one mode. The name and the button style are the
  // theme's whatever the mode.
  modeFields: [ "accent" , "second" , "background" , "surface" , "overrides" ],

  // Switches a theme to light or dark, in place. Its colors for the mode it
  // leaves are kept under that mode's name, and any it has for the mode it
  // goes to come back, so WSU switched to dark is WSU's dark colors and
  // switched back is its light ones again. A theme with nothing kept for the
  // new mode keeps its colors.
  setMode( theme , mode ) {
    if ( theme.mode === mode || !( mode in this.modes ) ) return theme;
    const leaving = {};
    this.modeFields.forEach( ( field ) => { if ( theme[ field ] !== undefined ) leaving[ field ] = theme[ field ]; } );
    const coming = theme[ mode ];
    if ( coming ) {
      this.modeFields.forEach( ( field ) => {
        if ( coming[ field ] !== undefined ) theme[ field ] = coming[ field ];
        else delete theme[ field ];
      } );
      theme.overrides = Object.assign( {} , theme.overrides || {} );
    }
    delete theme[ mode ];
    theme[ theme.mode ] = leaving;
    theme.mode = mode;
    return theme;
  },

  find( id ) {
    return this.list.find( function ( theme ) { return theme.id === id; } ) || this.list[ 0 ] || this.builtIn[ 0 ];
  },

  // What was kept in this browser: a whole theme, or (from before themes)
  // "light" or "dark", which is Default in that mode.
  restore() {
    let saved = null;
    try {
      const raw = window.localStorage.getItem( this.storageKey ) || "";
      if ( raw.charAt( 0 ) === "{" ) saved = JSON.parse( raw );
      else if ( raw in this.modes ) saved = this.setMode( this.copy( this.builtIn[ 0 ] ) , raw );
      else if ( raw !== "" ) saved = this.builtIn.find( function ( theme ) { return theme.id === raw; } ) || null;
    } catch ( error ) {
      // Storage blocked (private window, previews) or unreadable: stay light.
    }
    if ( !this.valid( saved ) ) saved = this.builtIn[ 0 ];
    this.restored = saved;
    this.list = this.builtIn.map( ( theme ) => ( theme.id === saved.id ? this.copy( saved ) : this.copy( theme ) ) );
    this.current = saved.id;
    this.apply( saved );
  },

  // Enough of a theme to paint, so a broken stored value never breaks the page.
  valid( theme ) {
    const hex = /^#[0-9a-f]{6}$/i;
    return Boolean( theme && typeof theme === "object" && typeof theme.id === "string"
      && ( theme.mode === "light" || theme.mode === "dark" )
      && hex.test( theme.accent || "" ) && hex.test( theme.second || "" ) );
  },

  // Default exactly as built in, in either mode: what style.css already draws.
  stock( theme ) {
    if ( theme.id !== this.builtIn[ 0 ].id ) return false;
    const built = this.setMode( this.copy( this.builtIn[ 0 ] ) , theme.mode );
    return JSON.stringify( this.colors( theme ) ) === JSON.stringify( this.colors( built ) )
      && JSON.stringify( this.buttonStyle( theme ) ) === JSON.stringify( this.buttonStyle( built ) );
  },

  // Keeps the theme in use for the next first paint. Plain Default in light is
  // kept as nothing at all, which is what a new browser starts with anyway.
  keep( theme ) {
    try {
      if ( theme.mode === "light" && this.stock( theme ) ) window.localStorage.removeItem( this.storageKey );
      else window.localStorage.setItem( this.storageKey , JSON.stringify( theme ) );
    } catch ( error ) {
      // Not remembered for next time, but still applied now.
    }
  },

  // The custom properties style.css reads, from colors().
  rookVars( colors ) {
    const vars = {
      "--fg": colors.text,
      "--fg-muted": this.asText( this.mix( colors.text , colors.surface , 62 ) , colors.surface ),
      "--bg": colors.background,
      "--card-bg": colors.surface,
      "--surface-2": colors.nested,
      "--border": colors.border,
      "--accent": colors.accent,
      "--accent-fg": colors.accentFg,
      "--accent-strong": colors.link,
      "--accent-soft": this.mix( colors.accent , colors.surface , 14 ),
      "--chrome": colors.chrome,
      "--nav-link": colors.navLink,
      "--nav-current": colors.second,
      "--nav-current-fg": colors.secondFg,
      "--led-ring": colors.ledRing,
      "--win-ring": colors.winRing,
    };
    this.variants.forEach( ( variant ) => {
      const name = this.buttonField( variant );
      vars[ "--" + variant ] = colors[ name ];
      vars[ "--" + variant + "-fg" ] = colors[ name + "Fg" ];
    } );
    return vars;
  },

  // Every property apply() may set, so a switch back to Default clears them.
  allVars() {
    const names = Object.keys( this.rookVars( this.colors( this.builtIn[ 0 ] ) ) );
    Object.keys( this.buttonChoices ).forEach( ( key ) => {
      Object.keys( this.buttonChoices[ key ] ).forEach( ( choice ) => {
        Object.keys( this.buttonChoices[ key ][ choice ] ).forEach( ( name ) => { if ( names.indexOf( name ) < 0 ) names.push( name ); } );
      } );
    } );
    return names;
  },

  // Paints a theme on the page, or (with an element) on that element alone:
  // the Display screen's preview.
  apply( theme , element ) {
    const root = element || document.documentElement;
    if ( theme.mode === "dark" ) root.setAttribute( "data-theme" , "dark" );
    else if ( element ) root.setAttribute( "data-theme" , "light" );
    else root.removeAttribute( "data-theme" );
    this.allVars().forEach( ( name ) => root.style.removeProperty( name ) );
    const colors = this.colors( theme );
    if ( element || !this.stock( theme ) ) {
      const vars = Object.assign( this.rookVars( colors ) , this.buttonStyle( theme ) );
      Object.keys( vars ).forEach( ( name ) => root.style.setProperty( name , vars[ name ] ) );
    }
    if ( element ) return;
    const meta = document.querySelector( "meta[name='theme-color']" );
    if ( meta ) meta.setAttribute( "content" , this.stock( theme ) ? this.modes[ theme.mode ].background : colors.background );
  },

  // A theme's button choices, each known one or the default.
  button( theme ) {
    const chosen = ( theme && theme.button ) || {};
    const result = {};
    Object.keys( this.buttonDefault ).forEach( ( key ) => {
      result[ key ] = chosen[ key ] in this.buttonChoices[ key ] ? chosen[ key ] : this.buttonDefault[ key ];
    } );
    return result;
  },

  // The custom properties style.css reads for every button's shape.
  buttonStyle( theme ) {
    const button = this.button( theme );
    return Object.assign( {} , ...Object.keys( button ).map( ( key ) => this.buttonChoices[ key ][ button[ key ] ] ) );
  },

  changed() {
    this.onChange.forEach( ( listener ) => listener() );
  },

  // The account's themes, from /api/me (or a save's reply). Someone whose
  // account never picked one keeps what this browser had, if it is on offer:
  // the dark choice from before themes is still dark.
  load( me ) {
    const kept = this.current;
    this.signedIn = true;
    this.custom = Boolean( me.themes_custom );
    this.list = Array.isArray( me.themes ) && me.themes.length > 0
      ? me.themes.map( ( theme ) => this.copy( theme ) )
      : this.builtIn.map( ( theme ) => this.named( theme ) );
    this.current = this.find( me.theme_id || kept ).id;
    const restored = this.restored;
    if ( !me.theme_id && !this.custom && restored && restored.id === this.current ) {
      this.list = this.list.map( ( theme ) => ( theme.id === restored.id ? Object.assign( this.copy( restored ) , { name: theme.name } ) : theme ) );
    }
    this.restored = null;
    this.show();
  },

  // Paints the theme in use and keeps it for next time.
  show() {
    const theme = this.find( this.current );
    this.apply( theme );
    this.keep( theme );
    this.changed();
  },

  // Uses another theme from the list, on this account.
  async pick( id ) {
    const previous = this.current;
    if ( previous === id ) return;
    this.current = this.find( id ).id;
    this.show();
    if ( !this.signedIn ) return;
    try {
      await Api.setTheme( this.current );
    } catch ( error ) {
      this.current = previous;
      this.show();
      throw error;
    }
  },

  // Saves someone's whole list (null: back to the defaults) and which one they
  // use, and takes the server's answer as the new state.
  async store( list , id ) {
    const reply = await Api.setThemes( list , id );
    this.load( reply );
  },

  // Every color the page needs, from a theme's few, before its overrides.
  // Fills keep the color as chosen. Anything read as text is pushed until it
  // is readable, on the nested card, the most tinted place it can land.
  derived( theme ) {
    const mode = this.modes[ theme.mode ] || this.modes.light;
    const background = theme.background || mode.background;
    const surface = theme.surface || mode.surface;
    const second = theme.second;
    const card = this.mix( second , surface , this.tints.surface );
    const nested = this.mix( second , surface , this.tints.nested );
    const chrome = this.mix( second , surface , this.tints.chrome );
    const result = {
      text: mode.text,
      accent: theme.accent,
      accentFg: this.readableOn( theme.accent ),
      second: second,
      secondFg: this.readableOn( second ),
      link: this.asText( theme.accent , nested ),
      navLink: this.asText( theme.accent , chrome ),
      background: this.mix( second , background , this.tints.page ),
      surface: card,
      nested: nested,
      chrome: chrome,
      // From the card, not the page: most borders sit on a card.
      border: this.mix( mode.text , card , 20 ),
      // The trick's outlines (Play settings switch them on): the mode's own.
      ledRing: mode.ledRing,
      winRing: mode.winRing,
    };
    const semantic = this.semantic[ theme.mode ] || this.semantic.light;
    this.variants.forEach( ( variant ) => {
      const name = this.buttonField( variant );
      const fill = this.asFill( this.mix( theme.accent , semantic[ variant ] , this.buttonHarmony ) );
      result[ name ] = fill;
      result[ name + "Fg" ] = this.readableOn( fill );
    } );
    return result;
  },

  // "danger" -> "btnDanger", the field of colors() for that button's fill.
  buttonField( variant ) {
    return "btn" + variant.charAt( 0 ).toUpperCase() + variant.slice( 1 );
  },

  // This color, moved darker (or lighter) until black or white reads on it.
  asFill( color ) {
    const label = this.readableOn( color );
    const away = label === "#ffffff" ? "#000000" : "#ffffff";
    for ( let step = 0 ; step <= 100 ; step += 2 ) {
      const tried = this.mix( away , color , step );
      if ( this.contrast( tried , label ) >= this.minContrast ) return tried;
    }
    return away;
  },

  // What the page uses: the worked-out colors, with the theme's overrides on top.
  colors( theme ) {
    const colors = this.derived( theme );
    Object.keys( theme.overrides || {} ).forEach( ( field ) => {
      if ( field in colors ) colors[ field ] = theme.overrides[ field ];
    } );
    return colors;
  },

  // The pairs that are too hard to read in this theme, worst first:
  // [ { name , front , back , ratio } ].
  check( theme ) {
    const colors = this.colors( theme );
    return Object.keys( this.pairs ).map( ( name ) => {
      const [ front , back ] = this.pairs[ name ];
      return { name: name , front: front , back: back , ratio: this.contrast( colors[ front ] , colors[ back ] ) };
    } ).filter( ( pair ) => pair.ratio < this.minContrast ).sort( ( a , b ) => a.ratio - b.ratio );
  },

  // percent of over, the rest under.
  mix( over , under , percent ) {
    const channel = function ( color , offset ) { return parseInt( color.slice( offset , offset + 2 ) , 16 ); };
    return "#" + [ 1 , 3 , 5 ].map( function ( offset ) {
      const mixed = channel( over , offset ) * percent / 100 + channel( under , offset ) * ( 100 - percent ) / 100;
      return Math.round( mixed ).toString( 16 ).padStart( 2 , "0" );
    } ).join( "" );
  },

  // Black or white, for a label on this color.
  readableOn( color ) {
    return this.contrast( color , "#ffffff" ) >= this.contrast( color , "#0b0d10" ) ? "#ffffff" : "#0b0d10";
  },

  // This color, moved toward black or white until text in it reads on ground.
  asText( color , ground ) {
    const away = this.luminance( ground ) > 0.4 ? "#000000" : "#ffffff";
    for ( let step = 0 ; step <= 100 ; step += 2 ) {
      const tried = this.mix( away , color , step );
      if ( this.contrast( tried , ground ) >= this.minContrast ) return tried;
    }
    return away;
  },

  // Relative luminance (WCAG) of #rrggbb.
  luminance( hex ) {
    const channel = function ( offset ) {
      const value = parseInt( hex.slice( offset , offset + 2 ) , 16 ) / 255;
      return value <= 0.03928 ? value / 12.92 : Math.pow( ( value + 0.055 ) / 1.055 , 2.4 );
    };
    return 0.2126 * channel( 1 ) + 0.7152 * channel( 3 ) + 0.0722 * channel( 5 );
  },

  contrast( first , second ) {
    const a = this.luminance( first ) , b = this.luminance( second );
    return ( Math.max( a , b ) + 0.05 ) / ( Math.min( a , b ) + 0.05 );
  },
};

// The danger button's two colors may be overridden, and its label must read.
Theme.variants.forEach( ( variant ) => {
  const name = Theme.buttonField( variant );
  Theme.fields.push( name , name + "Fg" );
  Theme.pairs[ "button_" + variant ] = [ name + "Fg" , name ];
} );

Theme.restore();

// Opened from the Home Screen on an iPad or iPhone (navigator.standalone is
// Apple's own flag for exactly that): iPadOS blurs a band under the clock
// that reaches further down than the safe area it reports, and blurs the top
// of the page with it. Marked here, before first paint, so style.css can
// start the page below that band without the page jumping.
if ( window.navigator.standalone === true ) document.documentElement.classList.add( "ios-app" );
