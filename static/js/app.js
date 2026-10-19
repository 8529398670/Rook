// Boot and routing. Load the language file, ask /api/me once, then show one
// of four screens based on the URL hash:
//
//   #/                 the lobby
//   #/t/<id>           a table
//   #/settings         account, theme, API keys, admin
//   #/games            the game history (#/games/<id> for one game)
//   #/join/<code>      a table's join link
//   #/watch/<code>     a table's watch link
//   #/signup/<cred>    a multi-use sign-up link
//
// The last three also work signed out: they are how people get in.
//
// Do not re-check the session anywhere else -- the server is the authority,
// and a second client-side notion of "am I logged in" is how the two drift
// apart. Each screen is its own file with enter() and leave(); this file only
// decides which one is showing.
const App = {
  me: null,
  screen: null,

  async init() {
    try {
      await I18n.load();
      I18n.apply();
    } catch ( error ) {
      // Without the language file every label is blank. This is the one
      // place that can afford a hardcoded string.
      document.body.textContent = "Could not load the language file.";
      return;
    }

    Cards.init();
    Sheet.init();
    Fullscreen.init();
    this.drawArt();

    let me = { authenticated: false };
    try {
      me = await Api.me();
    } catch ( error ) {
      Toast.show( I18n.get( "errors.network" ) , "error" );
    }

    this.me = me.authenticated ? me : null;
    Join.init();
    Signup.init();
    Table.init( this.me );
    if ( this.me ) {
      Settings.init( me );
      Profile.init( me );
      Lobby.init( me );
      Global.start( me );
    }
    window.addEventListener( "hashchange" , this.route.bind( this ) );
    this.route();
  },

  showSignedOut() {
    this.show( "screen-signed-out" );
    // The login route redirects here with ?login=invalid rather than
    // explaining what was wrong with the link -- see routes/auth.go.
    const params = new URLSearchParams( window.location.search );
    if ( params.get( "login" ) === "invalid" && I18n.get( "signed_out.invalid_link" ) !== "" ) {
      Dom.show( Dom.get( "invalid-link" ) , true );
    }
  },

  // Fill every [data-art] slot with its drawing.
  drawArt() {
    Dom.all( "[data-art]" ).forEach( function ( slot ) {
      Dom.clear( slot );
      switch ( slot.getAttribute( "data-art" ) ) {
        case "splash":
          slot.appendChild( Art.wordmarkSvg( "wordmark-art" ) );
          slot.appendChild( Art.birdSvg( "bird-art" ) );
          break;
        case "wordmark":
          slot.appendChild( Art.wordmarkSvg( "wordmark-art" ) );
          break;
        case "bird":
          slot.appendChild( Art.birdSvg( "bird-art" ) );
          break;
      }
    } );
  },

  route() {
    const hash = window.location.hash || "#/";
    const match = function ( prefix ) {
      const found = hash.match( new RegExp( "^#/" + prefix + "/([A-Za-z0-9_.-]+)$" ) );
      return found ? found[ 1 ] : null;
    };
    let next = Lobby;
    let argument = null;
    if ( match( "t" ) ) {
      next = Table;
      argument = { id: match( "t" ) };
    } else if ( match( "watch" ) ) {
      next = Table;
      argument = { watch: match( "watch" ) };
    } else if ( match( "join" ) ) {
      next = Join;
      argument = match( "join" );
    } else if ( match( "signup" ) ) {
      next = Signup;
      argument = match( "signup" );
    } else if ( hash === "#/settings" ) {
      next = Settings;
    } else if ( hash === "#/games" ) {
      next = Games;
    } else if ( match( "games" ) ) {
      next = Games;
      argument = match( "games" );
    }

    // Signed out, only the link screens are open.
    const open = next === Join || next === Signup || ( next === Table && argument.watch );
    if ( !this.me && !open ) {
      if ( this.screen ) this.screen.leave();
      this.screen = null;
      this.showSignedOut();
      return;
    }

    if ( this.screen && this.screen !== next ) this.screen.leave();
    Sheet.close( true );
    document.title = I18n.get( "app.title" ); // a table puts its own name here
    this.screen = next;
    this.show( next.screenId );
    next.enter( argument );
  },

  go( hash ) {
    if ( window.location.hash === hash ) {
      this.route();
      return;
    }
    window.location.hash = hash;
  },

  show( id ) {
    Dom.all( ".screen" ).forEach( function ( screen ) { screen.hidden = screen.id !== id; } );
    document.body.setAttribute( "data-screen" , id );
    window.scrollTo( 0 , 0 );
  },

  // Kept for the template's keys.js, which reports failures through here.
  showError( message ) {
    Toast.show( message || I18n.get( "errors.generic" ) , "error" );
  },
};

document.addEventListener( "DOMContentLoaded" , function () {
  App.init();
} );
