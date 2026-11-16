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
  // The Ohio State University. Primary: scarlet, gray and white. Scarlet is
  // for what you press; it has shades but no tints (nothing lighter).
  osu: {
    scarlet: "#ba0c2f",
    gray: "#a7b1b7",
    white: "#ffffff",
    scarlet_dark_40: "#70071c",
    scarlet_dark_60: "#4a0513",
    gray_light_20: "#bfc6cb",
    gray_light_40: "#cfd4d8",
    gray_light_60: "#dfe3e5",
    gray_light_80: "#eff1f2",
    gray_light_90: "#f6f7f8",
    gray_dark_20: "#868e92",
    gray_dark_40: "#646a6e",
    gray_dark_60: "#3f4443",
    gray_dark_80: "#212325",
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
  //
  // A theme added after the first ones carries since: one more than the
  // highest so far. A saved list remembers the highest it has seen
  // (builtInsSeen), and withNewBuiltIns adds any newer one to it, so a new
  // theme reaches people who already made their own list. Never reuse or
  // lower a since; moving a theme in this list does not move it in saved ones.
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
    { id: "cerise" , mode: "dark" , accent: "#f06595" , second: "#ffa8d0" , background: "#1f0d16" , surface: "#2a1220" ,
      light: { accent: "#b0164c" , second: "#e64980" , background: "#fdf2f6" , surface: "#ffffff" } },
    { id: "pink" , mode: "light" , accent: "#c2255c" , second: "#6b3fc0" , background: "#fdf0f5" , surface: "#ffffff" ,
      dark: { accent: "#f783ac" , second: "#b197fc" , background: "#1f1018" , surface: "#2a1621" } },
    // Wright State University: Green on Off White; switched to dark, Gold on Black with Dark Green panels.
    { id: "wsu" , mode: "light" , accent: ThemePalettes.wsu.green , second: ThemePalettes.wsu.gold , background: ThemePalettes.wsu.off_white , surface: "#ffffff" ,
      dark: { accent: ThemePalettes.wsu.gold , second: ThemePalettes.wsu.yellow , background: ThemePalettes.wsu.black , surface: ThemePalettes.wsu.dark_green } },
    // The Ohio State University: Scarlet on Gray Light 90 with Gray Dark 40 for
    // where you are; switched to dark, Scarlet on Gray Dark 80 with Gray Dark 60 panels.
    // Text is the gray shades; dark links and outline buttons are gray tints,
    // never a scarlet tint (no scarlet reads on a dark card).
    { id: "osu" , since: 1 , mode: "light" , accent: ThemePalettes.osu.scarlet , second: ThemePalettes.osu.gray_dark_40 , background: ThemePalettes.osu.gray_light_90 , surface: ThemePalettes.osu.white ,
      overrides: { text: ThemePalettes.osu.gray_dark_80 } ,
      dark: { accent: ThemePalettes.osu.scarlet , second: ThemePalettes.osu.gray , background: ThemePalettes.osu.gray_dark_80 , surface: ThemePalettes.osu.gray_dark_60 ,
        overrides: { text: ThemePalettes.osu.gray_light_60 , link: ThemePalettes.osu.gray_light_40 , navLink: ThemePalettes.osu.gray_light_40 ,
          accentText: ThemePalettes.osu.gray_light_40 } } },
    { id: "midnight" , mode: "dark" , accent: "#8c7cff" , second: "#ffb86b" , background: "#0e1020" , surface: "#171a30" ,
      light: { accent: "#5b4bd6" , second: "#c26a00" , background: "#f3f3fb" , surface: "#ffffff" } },
    { id: "slate" , mode: "dark" , accent: "#4fb3a9" , second: "#e6b450" , background: "#15191e" , surface: "#1d232a" ,
      light: { accent: "#2b7a72" , second: "#a0711a" , background: "#f2f4f6" , surface: "#ffffff" } },
  ],

  // What a theme leaves out: Rook's own page, card and text, and the outlines
  // on the card that led the trick and the one that won it, as in style.css.
  // Your team and the other team too, as style.css has them.
  modes: {
    light: { background: "#f4eee3" , surface: "#fffdf8" , text: "#1f1d1a" , ledRing: "#111111" , winRing: "#cba052" ,
      us: "#157f92" , them: "#c8582c" },
    dark: { background: "#111416" , surface: "#1a1e22" , text: "#ecebe6" , ledRing: "#000000" , winRing: "#cba052" ,
      us: "#4cc3d6" , them: "#f19166" },
  },
  // How much of a team's color tints its soft ground (score chips, summaries).
  teamSoft: { light: 16 , dark: 24 },

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
  // names, server/models/themes.go). accentText is kept for the template's
  // themes (OSU dark sets it); Rook draws no outline primary button with it.
  fields: [ "text" , "accent" , "accentFg" , "accentText" , "second" , "secondFg" , "link" , "navLink" ,
    "background" , "surface" , "nested" , "chrome" , "border" , "ledRing" , "winRing" ,
    // The game screen: the felt and its words, the panels on it (seat names,
    // the auction, the waiting room), the bar across the top, the tray and
    // hand along the bottom, and the two teams.
    "table" , "tableText" , "tablePanel" , "tablePanelText" , "tableBar" , "tableBarText" , "tray" , "trayText" ,
    "us" , "them" ],

  // Pairs that must stay readable: [ front , back ] fields of colors(). Names
  // are language.yaml keys (theme.check_<name>).
  pairs: {
    table_text: [ "tableText" , "table" ],
    table_panel: [ "tablePanelText" , "tablePanel" ],
    table_bar: [ "tableBarText" , "tableBar" ],
    table_tray: [ "trayText" , "tray" ],
    text_card: [ "text" , "surface" ],
    text_nested: [ "text" , "nested" ],
    link: [ "link" , "nested" ],
    button: [ "accentFg" , "accent" ],
    active_nav: [ "secondFg" , "second" ],
    sidebar_link: [ "navLink" , "chrome" ],
  },

  // The game table's looks; the server checks the same names
  // (ThemeTableLooks, server/models/themes.go).
  // A theme's table is { look } or { look: "image" , image , x , y , dim }:
  // a picture (backgrounds.js) placed by its focal point, x and y in
  // percent, and darkened by dim percent. No table: maple in light mode,
  // baize in dark -- the stylesheet's own. Names: theme.table_look_<look>.
  tableLooks: [ "maple" , "baize" , "blue" , "red" , "walnut" , "slate" , "plain" ],
  // What each look draws on the felt (table.css's .felt layers): its color
  // and the words on it, which a theme can still override (table,
  // tableText), and its plank seams, grain and glow, which it cannot. Maple
  // and baize are style.css's own light and dark. Plain is the theme's own
  // nested card and text; a picture sits on dark, with white words.
  tablePalettes: {
    maple: { table: "#ead3aa" , text: "#3a2a16" , seam: "rgba(124, 78, 28, 0.13)" , grain: "rgba(124, 78, 28, 0.05)" , glow: "rgba(255, 252, 240, 0.55)" },
    baize: { table: "#1f4a3b" , text: "#e6f1ea" , seam: "transparent" , grain: "rgba(255, 255, 255, 0.025)" , glow: "rgba(140, 220, 180, 0.12)" },
    blue: { table: "#1d3f66" , text: "#e8f0fa" , seam: "transparent" , grain: "rgba(255, 255, 255, 0.03)" , glow: "rgba(150, 190, 255, 0.15)" },
    red: { table: "#6b1f26" , text: "#fbecea" , seam: "transparent" , grain: "rgba(255, 255, 255, 0.03)" , glow: "rgba(255, 170, 150, 0.13)" },
    walnut: { table: "#4a2f1d" , text: "#f6eadb" , seam: "rgba(0, 0, 0, 0.24)" , grain: "rgba(0, 0, 0, 0.09)" , glow: "rgba(255, 210, 160, 0.12)" },
    slate: { table: "#30363b" , text: "#e9edf0" , seam: "transparent" , grain: "rgba(255, 255, 255, 0.02)" , glow: "rgba(220, 230, 240, 0.09)" },
    plain: { table: null , text: null , seam: "transparent" , grain: "transparent" , glow: "transparent" },
    image: { table: "#2b2b2b" , text: "#ffffff" , seam: "transparent" , grain: "transparent" , glow: "transparent" },
  },
  tableDimMax: 70,
  tableVars: [ "--table-image" , "--table-pos" , "--table-dim" ],

  // A theme's table, tidied, or null for the mode's own.
  table( theme ) {
    const table = theme && theme.table;
    if ( !table || typeof table !== "object" ) return null;
    const clamp = ( value , low , high , fallback ) => {
      const number = Number( value );
      return Number.isFinite( number ) ? Math.min( high , Math.max( low , Math.round( number ) ) ) : fallback;
    };
    if ( table.look === "image" ) {
      if ( !/^[0-9a-f]{24}$/.test( table.image || "" ) ) return null;
      return { look: "image" , image: table.image , x: clamp( table.x , 0 , 100 , 50 ) , y: clamp( table.y , 0 , 100 , 50 ) ,
        dim: clamp( table.dim , 0 , this.tableDimMax , 0 ) };
    }
    return this.tableLooks.indexOf( table.look ) >= 0 ? { look: table.look } : null;
  },

  // The look a table shows: its own, or the mode's.
  tableLook( theme ) {
    const table = this.table( theme );
    return table ? table.look : ( theme.mode === "dark" ? "baize" : "maple" );
  },

  // The felt's own custom properties for a theme as worked out, before any
  // override: for the look swatches in the editor.
  feltVars( theme ) {
    const derived = this.derived( theme );
    const palette = this.tablePalettes[ this.tableLook( theme ) ];
    return { "--table": derived.table , "--table-fg": derived.tableText ,
      "--table-seam": palette.seam , "--table-grain": palette.grain , "--table-glow": palette.glow };
  },

  // Paints a theme's table on an element: data-table-look, and for a
  // picture, where it is and how dark. The page's own (no look) is the
  // attribute being absent; a preview always names one (always). The
  // look's colors come with the rest, from rookVars.
  paintTable( theme , root , always ) {
    const table = this.table( theme );
    this.tableVars.forEach( ( name ) => root.style.removeProperty( name ) );
    if ( table || always ) root.setAttribute( "data-table-look" , table ? table.look : this.tableLook( theme ) );
    else root.removeAttribute( "data-table-look" );
    if ( !table || table.look !== "image" ) return;
    root.style.setProperty( "--table-image" , "url(\"/api/backgrounds/" + table.image + "\")" );
    root.style.setProperty( "--table-pos" , table.x + "% " + table.y + "%" );
    root.style.setProperty( "--table-dim" , table.dim + "%" );
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
    delete copy.since;
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
    const table = this.table( theme );
    if ( table ) copy.table = table;
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
    if ( theme.id !== this.builtIn[ 0 ].id || this.table( theme ) ) return false;
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

  // A quieter line of text in this color, still readable on ground.
  muted( text , ground ) {
    return this.asText( this.mix( text , ground , 62 ) , ground );
  },

  // The custom properties style.css and table.css read, from colors(); the
  // theme says which look the felt's seams and grain come from.
  rookVars( colors , theme ) {
    const palette = this.tablePalettes[ this.tableLook( theme ) ];
    const soft = this.teamSoft[ theme.mode ] || this.teamSoft.light;
    // The bar while an admin holds the game: tinted with the theme's main
    // color, or its second where the bar is already the main one (WSU, OSU).
    const held = this.contrast( colors.accent , colors.tableBar ) >= 1.6 ? colors.accent : colors.second;
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
      // The game screen (table.css): every one is set, so a theme's table
      // is all its own whatever the mode's stylesheet says.
      "--table": colors.table,
      "--table-fg": colors.tableText,
      "--table-seam": palette.seam,
      "--table-grain": palette.grain,
      "--table-glow": palette.glow,
      "--table-panel": "color-mix(in srgb, " + colors.tablePanel + " 92%, transparent)",
      "--table-panel-fg": colors.tablePanelText,
      "--table-panel-muted": this.muted( colors.tablePanelText , colors.tablePanel ),
      "--game-bar": colors.tableBar,
      "--game-bar-fg": colors.tableBarText,
      "--game-bar-muted": this.muted( colors.tableBarText , colors.tableBar ),
      "--game-tray": colors.tray,
      "--game-tray-fg": colors.trayText,
      "--game-tray-muted": this.muted( colors.trayText , colors.tray ),
      "--game-held": this.mix( held , colors.tableBar , 16 ),
      "--game-held-line": held,
      "--us": colors.us,
      "--us-soft": this.mix( colors.us , colors.surface , soft ),
      "--them": colors.them,
      "--them-soft": this.mix( colors.them , colors.surface , soft ),
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
    const names = Object.keys( this.rookVars( this.colors( this.builtIn[ 0 ] ) , this.builtIn[ 0 ] ) );
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
      const vars = Object.assign( this.rookVars( colors , theme ) , this.buttonStyle( theme ) );
      Object.keys( vars ).forEach( ( name ) => root.style.setProperty( name , vars[ name ] ) );
    }
    this.paintTable( theme , root , Boolean( element ) );
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
      ? this.withNewBuiltIns( me.themes.map( ( theme ) => this.copy( theme ) ) , me.themes_builtins || 0 )
      : this.builtIn.map( ( theme ) => this.named( theme ) );
    this.current = this.find( me.theme_id || kept ).id;
    const restored = this.restored;
    if ( !me.theme_id && !this.custom && restored && restored.id === this.current ) {
      // Its colors and mode as this browser had them, but the list's table:
      // a picture the defaults put on it is not something a browser kept.
      this.list = this.list.map( ( theme ) => {
        if ( theme.id !== restored.id ) return theme;
        const merged = Object.assign( this.copy( restored ) , { name: theme.name } );
        if ( theme.table ) merged.table = theme.table;
        else delete merged.table;
        return merged;
      } );
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
    const reply = await Api.setThemes( list , id , this.builtInsSeen() );
    this.load( reply );
  },

  // Which built-in themes a list saved now has seen: the highest since.
  builtInsSeen() {
    return Math.max( this.gameSince , ...this.builtIn.map( ( theme ) => theme.since || 0 ) );
  },

  // The "builtins" at which the built-in themes got their game screen colors
  // (ThemeGames, below). A list saved before that gets them too: see
  // withGame. Keep it above every since in builtIn.
  gameSince: 2,

  // The game screen's fields: a theme that sets any of them, or a table,
  // has made its own choice there.
  gameFields: [ "table" , "tableText" , "tablePanel" , "tablePanelText" , "tableBar" , "tableBarText" , "tray" , "trayText" , "us" , "them" ],

  // A theme with its built-in game screen colors (ThemeGames) added, in
  // place, for each mode it carries -- unless it has a table or any game
  // color of its own already, or is not one of the built-in themes.
  withGame( theme ) {
    const game = typeof ThemeGames === "undefined" ? null : ThemeGames[ theme.id ];
    if ( !game || theme.table ) return theme;
    const modes = [ theme , theme.light , theme.dark ].filter( Boolean );
    if ( modes.some( ( colors ) => this.gameFields.some( ( field ) => colors.overrides && colors.overrides[ field ] ) ) ) return theme;
    theme.table = { look: game.look };
    theme.overrides = Object.assign( {} , game[ theme.mode ] , theme.overrides || {} );
    const other = theme.mode === "dark" ? "light" : "dark";
    if ( theme[ other ] ) theme[ other ].overrides = Object.assign( {} , game[ other ] , theme[ other ].overrides || {} );
    return theme;
  },

  // A saved list with every built-in theme newer than seen added, each just
  // after the built-in before it (or at the end), while there is room (40, the
  // server's limit). One already there by id is left as it is.
  withNewBuiltIns( list , seen ) {
    const result = seen < this.gameSince ? list.map( ( theme ) => this.withGame( theme ) ) : list.slice();
    this.builtIn.forEach( ( theme , index ) => {
      if ( ( theme.since || 0 ) <= seen || result.length >= 40 ) return;
      if ( result.some( ( item ) => item.id === theme.id ) ) return;
      let at = result.length;
      for ( let back = index - 1; back >= 0; back -= 1 ) {
        const found = result.findIndex( ( item ) => item.id === this.builtIn[ back ].id );
        if ( found >= 0 ) {
          at = found + 1;
          break;
        }
      }
      result.splice( at , 0 , this.named( theme ) );
    } );
    return result;
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
      // The outline button's label and border, read on the card.
      accentText: this.asText( theme.accent , card ),
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
    // The game screen, as the page has it until a theme says otherwise: the
    // felt as its look draws it, the panels on it and the bar as cards, the
    // tray and hand on the page, and the mode's two teams.
    const palette = this.tablePalettes[ this.tableLook( theme ) ];
    result.table = palette.table || nested;
    result.tableText = palette.text || this.asText( mode.text , nested );
    result.tablePanel = card;
    result.tablePanelText = mode.text;
    result.tableBar = card;
    result.tableBarText = mode.text;
    result.tray = result.background;
    result.trayText = mode.text;
    result.us = mode.us;
    result.them = mode.them;
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

// Each built-in theme's game screen (Default keeps the stylesheet's own):
// a table look, and per mode the felt and its words, and the two teams --
// your team in the theme's main color, the other in its second, or another
// that stays apart where those two are too alike. WSU and OSU also put their
// school color on the bar across the top. They are overrides like any other,
// so the editor shows them and Reset puts the worked-out color back.
const ThemeGames = {
  ocean: { look: "blue" ,
    light: { table: "#b9dbe3" , tableText: "#0a3440" , us: "#0b7285" , them: "#bd5418" } ,
    dark: { table: "#0e3b48" , tableText: "#def3f7" , us: "#3bc9db" , them: "#ff922b" } },
  forest: { look: "baize" ,
    light: { table: "#b7d8b4" , tableText: "#183a1d" , us: "#28843b" , them: "#9a6b0c" } ,
    dark: { table: "#1d3f24" , tableText: "#e2f3e0" , us: "#69db7c" , them: "#e0b84c" } },
  sepia: { look: "walnut" ,
    light: { table: "#e4cfa8" , tableText: "#3d2a12" , us: "#9c5a1c" , them: "#2f6f6a" } ,
    dark: { table: "#3b2a1a" , tableText: "#f1e4cf" , us: "#e0a15e" , them: "#6cc4bb" } },
  grape: { look: "slate" ,
    light: { table: "#d9cff3" , tableText: "#2e1f5c" , us: "#7048c8" , them: "#c0267e" } ,
    dark: { table: "#2a2145" , tableText: "#ece6ff" , us: "#b197fc" , them: "#f783ac" } },
  cerise: { look: "red" ,
    light: { table: "#f3c6d6" , tableText: "#4a0f27" , us: "#b0164c" , them: "#a85d00" } ,
    dark: { table: "#4a1530" , tableText: "#fde8f1" , us: "#f06595" , them: "#ffc078" } },
  pink: { look: "red" ,
    light: { table: "#f6c9da" , tableText: "#4b0f26" , us: "#c2255c" , them: "#6b3fc0" } ,
    dark: { table: "#4a1a2e" , tableText: "#fde6ef" , us: "#f783ac" , them: "#b197fc" } },
  wsu: { look: "baize" ,
    light: { table: ThemePalettes.wsu.green , tableText: ThemePalettes.wsu.off_white ,
      tableBar: ThemePalettes.wsu.green , tableBarText: "#ffffff" ,
      // Deep Gold a shade darker, so the other team's score reads on its chip.
      us: ThemePalettes.wsu.green , them: "#8c6a2f" } ,
    dark: { table: ThemePalettes.wsu.dark_green , tableText: ThemePalettes.wsu.faded_gold ,
      tableBar: ThemePalettes.wsu.dark_green , tableBarText: ThemePalettes.wsu.yellow ,
      // Yellow, and Leaf lightened: both read on a dark chip, and stay apart.
      us: ThemePalettes.wsu.yellow , them: "#b5d38c" } },
  osu: { look: "slate" ,
    light: { table: ThemePalettes.osu.gray_light_40 , tableText: ThemePalettes.osu.gray_dark_80 ,
      tableBar: ThemePalettes.osu.scarlet , tableBarText: ThemePalettes.osu.white ,
      us: ThemePalettes.osu.scarlet , them: ThemePalettes.osu.gray_dark_40 } ,
    dark: { table: ThemePalettes.osu.gray_dark_60 , tableText: ThemePalettes.osu.gray_light_80 ,
      tableBar: ThemePalettes.osu.scarlet , tableBarText: ThemePalettes.osu.white ,
      // Scarlet has no tints, but no shade of it reads on a dark chip: the
      // one place it is lightened, as dark mode already does with its grays.
      us: "#ff9aab" , them: ThemePalettes.osu.gray_light_20 } },
  midnight: { look: "blue" ,
    light: { table: "#cfd0f2" , tableText: "#23235c" , us: "#5b4bd6" , them: "#a85a00" } ,
    dark: { table: "#1a1f45" , tableText: "#e6e6ff" , us: "#8c7cff" , them: "#ffb86b" } },
  slate: { look: "slate" ,
    light: { table: "#cdd5dc" , tableText: "#1f2a33" , us: "#2b7a72" , them: "#a0711a" } ,
    dark: { table: "#26313a" , tableText: "#e6edf2" , us: "#4fb3a9" , them: "#e6b450" } },
};
Theme.builtIn.forEach( ( theme ) => Theme.withGame( theme ) );

Theme.restore();

// Opened from the Home Screen on an iPad or iPhone (navigator.standalone is
// Apple's own flag for exactly that): iPadOS blurs a band under the clock
// that reaches further down than the safe area it reports, and blurs the top
// of the page with it. Marked here, before first paint, so style.css can
// start the page below that band without the page jumping.
if ( window.navigator.standalone === true ) document.documentElement.classList.add( "ios-app" );
