// "Where you're signed in", a card on the Account settings page: every
// device with a live session on this account, and a way to sign one
// out from here.
//
// Signing a device out is never undoable: it is the button for a lost phone,
// and an undo would let the session back in. So it asks first. This device has
// no button here; that is what Log out is for.
const Sessions = {
  // What the confirm sheet will do once its button is pressed.
  pending: null,

  init() {
    Dom.get( "sessions-revoke-others" ).addEventListener( "click" , () => {
      this.confirm( I18n.get( "sessions.revoke_others_heading" ) , I18n.get( "sessions.revoke_others_confirm" ) ,
        I18n.get( "sessions.revoke_others_button" ) , async () => { await Api.revokeOtherSessions(); } );
    } );
    Dom.get( "session-confirm-button" ).addEventListener( "click" , async () => {
      const action = this.pending;
      this.pending = null;
      Sheet.close( true );
      if ( !action ) return;
      try {
        await action();
      } catch ( error ) {
        Toast.error( error );
      }
      await this.refresh();
    } );
  },

  confirm( heading , body , button , action ) {
    Dom.text( Dom.get( "session-confirm-heading" ) , heading );
    Dom.text( Dom.get( "session-confirm-body" ) , body );
    Dom.text( Dom.get( "session-confirm-button" ) , button );
    this.pending = action;
    Sheet.open( "sheet-confirm-session" , { onClose: () => { this.pending = null; } } );
  },

  // A short name out of a User-Agent header. It only has to help someone tell
  // their own devices apart; the order matters, since Edge and Chrome both
  // say Safari, and Android says Linux.
  browser( agent ) {
    const known = [ [ /Edg\// , "Edge" ] , [ /OPR\// , "Opera" ] , [ /Firefox\// , "Firefox" ] , [ /Chrome\// , "Chrome" ] , [ /Safari\// , "Safari" ] ];
    const match = known.find( function ( entry ) { return entry[ 0 ].test( agent ); } );
    return match ? match[ 1 ] : "";
  },

  system( agent ) {
    const known = [ [ /Android/ , "Android" ] , [ /iPhone|iPad/ , "iOS" ] , [ /Mac OS X/ , "macOS" ] , [ /Windows/ , "Windows" ] , [ /Linux/ , "Linux" ] ];
    const match = known.find( function ( entry ) { return entry[ 0 ].test( agent ); } );
    return match ? match[ 1 ] : "";
  },

  device( session ) {
    const agent = session.user_agent || "";
    const browser = this.browser( agent );
    const system = this.system( agent );
    if ( browser && system ) return I18n.format( "sessions.device_name" , { browser: browser , system: system } );
    return browser || system || agent || I18n.get( "sessions.unknown_device" );
  },

  when( value ) {
    if ( !value ) return I18n.get( "sessions.unknown" );
    const date = new Date( value );
    if ( Number.isNaN( date.getTime() ) ) return I18n.get( "sessions.unknown" );
    return date.toLocaleString( undefined , { dateStyle: "medium" , timeStyle: "short" } );
  },

  async refresh() {
    const list = Dom.get( "sessions-list" );
    let page;
    try {
      page = await Api.sessions();
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    const sessions = page.sessions || [];
    Dom.clear( list );
    sessions.forEach( ( session ) => list.appendChild( this.row( session ) ) );
    Dom.show( Dom.get( "sessions-empty" ) , sessions.length === 0 );
    Dom.show( Dom.get( "sessions-revoke-others" ) , sessions.length > 1 );
  },

  row( session ) {
    const name = session.label || this.device( session );
    const where = session.ip_name ? session.ip_name + " · " + session.ip : ( session.ip || I18n.get( "sessions.unknown" ) );
    const lines = [
      Dom.el( "div" , { class: "session-name" , children: [
        Dom.el( "span" , { text: name } ),
        session.current ? Dom.el( "span" , { class: "session-current" , text: I18n.get( "sessions.this_device" ) } ) : null,
      ].filter( Boolean ) } ),
    ];
    if ( session.label ) lines.push( Dom.el( "div" , { class: "muted" , text: this.device( session ) } ) );
    lines.push( Dom.el( "div" , { class: "muted" , text: I18n.format( "sessions.details" , {
      where: where,
      seen: this.when( session.last_seen_at ),
      signed_in: this.when( session.created_at ),
    } ) } ) );
    const children = [ Dom.el( "div" , { class: "session-main" , children: lines } ) ];
    if ( !session.current ) {
      children.push( Dom.el( "button" , {
        class: "secondary small",
        text: I18n.get( "sessions.revoke_button" ),
        attrs: { type: "button" , "aria-label": I18n.format( "sessions.revoke_label" , { name: name } ) },
        on: { click: () => this.confirm( I18n.get( "sessions.revoke_heading" ) , I18n.format( "sessions.revoke_confirm" , { name: name } ) ,
          I18n.get( "sessions.revoke_button" ) , async () => { await Api.revokeSession( session.id ); } ) },
      } ) );
    }
    return Dom.el( "li" , { class: "session-row" , children: children } );
  },
};
