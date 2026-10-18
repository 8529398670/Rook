// A table's join link (#/join/<code>). Signed in: take a free seat and go
// to the table. Signed out: type a name and get an account on the spot --
// only while an admin allows that -- or be told to ask for a login link.
const Join = {
  screenId: "screen-join",
  code: null,

  init() {
    Dom.get( "join-signup-form" ).addEventListener( "submit" , this.onSignup.bind( this ) );
  },

  showError( error ) {
    const box = Dom.get( "join-error" );
    Dom.text( box , I18n.error( error && error.code ) );
    box.hidden = false;
  },

  async enter( code ) {
    this.code = code;
    [ "join-signup-form" , "join-closed" , "join-error" , "join-working" ].forEach( function ( id ) { Dom.get( id ).hidden = true; } );
    Dom.text( Dom.get( "join-table-name" ) , "" );
    Dom.text( Dom.get( "join-status" ) , "" );

    let info = null;
    try {
      info = await Api.joinInfo( code );
    } catch ( error ) {
      this.showError( error );
      return;
    }
    Dom.text( Dom.get( "join-table-name" ) , info.name );
    let status = I18n.get( "join.no_seats" );
    if ( info.open_seats > 0 ) {
      status = I18n.format( "join.open_seats" , { count: info.open_seats } );
    } else if ( info.bot_seats > 0 ) {
      status = I18n.get( "join.bot_seats" );
    }
    Dom.text( Dom.get( "join-status" ) , status );

    if ( App.me ) {
      Dom.get( "join-working" ).hidden = false;
      try {
        const joined = await Api.joinTable( code );
        // Asked to take over a bot: the table shows them waiting on the host.
        if ( joined.requested ) Toast.show( I18n.get( "takeover.sent" ) );
        App.go( "#/t/" + joined.table_id );
      } catch ( error ) {
        Dom.get( "join-working" ).hidden = true;
        this.showError( error );
      }
      return;
    }
    Dom.get( "join-signup-form" ).hidden = !info.signup_allowed;
    Dom.get( "join-closed" ).hidden = info.signup_allowed;
  },

  leave() {},

  async onSignup( event ) {
    event.preventDefault();
    const name = Dom.get( "join-name" ).value.trim();
    if ( !name ) return;
    try {
      const joined = await Api.joinSignup( this.code , name );
      // A fresh session cookie is set now; reload so the whole app starts
      // over as the new account, at the table.
      window.location.hash = "#/t/" + joined.table_id;
      window.location.reload();
    } catch ( error ) {
      this.showError( error );
    }
  },
};
