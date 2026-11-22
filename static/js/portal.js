// The owner's ProjectPortal card, beside All API keys in Settings (portal
// contract v3). ProjectPortal works through a service account of its own,
// never the owner's key: Connect makes that account (when there is none by
// the name given) and one admin key for it that may make keys and never
// sign-in links, shown once. The status lists its live keys and when they
// expire, and Disconnect revokes them all, and with them everything they
// made.
//
// Only the owner has the card: the routes are the owner's (role=OWNER), and
// answer anyone else 404. settings.js calls Portal.init( me ) once it knows
// who is signed in.
const Portal = {
  me: null,
  wired: false,

  async init( me ) {
    this.me = me;
    if ( me.api_keys_enabled === false || !me.owner ) return;
    Dom.show( Dom.get( "portal-card" ) , true );
    if ( !this.wired ) {
      this.wired = true;
      Dom.get( "portal-connect-form" ).addEventListener( "submit" , this.onConnect.bind( this ) );
      Dom.get( "copy-portal-key-button" ).addEventListener( "click" , function () {
        Dom.copy( Dom.get( "portal-key-value" ) , Dom.get( "portal-key-copied" ) );
      } );
    }
    await this.refresh();
  },

  async refresh() {
    let page;
    try {
      page = await Api.request( "/api/owner/portal" );
    } catch ( error ) {
      App.showError( error.message );
      return;
    }
    const list = Dom.get( "portal-status" );
    Dom.clear( list );
    const keys = [];
    page.accounts.forEach( function ( account ) {
      account.keys.forEach( function ( key ) { keys.push( { account: account , key: key } ); } );
    } );
    if ( keys.length === 0 ) {
      list.appendChild( Dom.el( "li" , { class: "muted" , text: I18n.get( "portal.none" ) } ) );
    }
    // A key with under 30 days left is flagged, so it is replaced in time.
    const soon = Date.now() + 30 * 24 * 60 * 60 * 1000;
    keys.forEach( function ( item ) {
      const expires = item.key.expires_at ? new Date( item.key.expires_at ) : null;
      const warning = expires !== null && expires.getTime() < soon;
      const when = expires === null ? I18n.get( "portal.never_expires" ) : I18n.format( "portal.expires" , { date: expires.toLocaleDateString() } );
      list.appendChild( Dom.el( "li" , {
        children: [
          Dom.el( "strong" , { text: item.account.display_name } ),
          Dom.el( "span" , { class: "muted" , text: " " + item.key.name + " " } ),
          Dom.el( "span" , { class: "log-badge " + ( warning ? "log-badge-warning" : "log-badge-info" ) , text: when } ),
        ],
      } ) );
    } );
    const button = Dom.get( "portal-disconnect-button" );
    Dom.show( button , keys.length > 0 );
    button.onclick = () => this.onDisconnect( page.accounts );
  },

  async onConnect( event ) {
    event.preventDefault();
    const name = Dom.get( "portal-name" ).value.trim() || "ProjectPortal";
    const days = Number( Dom.get( "portal-days" ).value );
    try {
      const created = await Api.post( "/api/owner/portal/connect" , { name: name , days: days } );
      Dom.text( Dom.get( "portal-key-value" ) , created.credential );
      Dom.show( Dom.get( "portal-key-box" ) , true );
      await this.refresh();
      if ( typeof Keys !== "undefined" && this.me.owner ) await Keys.refreshAll();
    } catch ( error ) {
      App.showError( error.message );
    }
  },

  async onDisconnect( accounts ) {
    // An empty confirm message means "do not ask", as everywhere in
    // language.yaml.
    const question = I18n.get( "portal.disconnect_confirm" );
    if ( question !== "" && !window.confirm( question ) ) return;
    try {
      for ( const account of accounts ) {
        await Api.post( "/api/owner/portal/disconnect" , { user_id: account.id } );
      }
      Dom.show( Dom.get( "portal-key-box" ) , false );
      await this.refresh();
      if ( typeof Keys !== "undefined" && this.me.owner ) await Keys.refreshAll();
    } catch ( error ) {
      App.showError( error.message );
    }
  },
};
