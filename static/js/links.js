// Settings panels about how people get in and what watchers see: the
// app-wide game settings (the owner only), and multi-use sign-up links, which
// anyone allowed to invite may make. An admin sees everyone's links, with
// who made each; anyone else only their own.
const Links = {
  settingsReady: false,
  linksReady: false,
  showMaker: false,

  // The pauses on offer after a finished trick, in ms -- the same list as
  // models.TrickPauses on the server.
  trickPauses: [ 500 , 800 , 1100 , 1500 , 2000 , 2500 , 3000 , 4000 , 5000 ],

  pauseText( ms ) {
    return I18n.format( "trick_pause.seconds" , { seconds: ( ms / 1000 ).toLocaleString( undefined , { maximumFractionDigits: 1 } ) } );
  },

  // Fill a select with the pauses. With defaultMs, the first choice is
  // "Default (x s)", value 0 -- a table following the app-wide setting.
  fillPauses( select , defaultMs ) {
    Dom.clear( select );
    if ( defaultMs ) {
      select.appendChild( Dom.el( "option" , { text: I18n.format( "trick_pause.default" , { time: this.pauseText( defaultMs ) } ) , attrs: { value: "0" } } ) );
    }
    this.trickPauses.forEach( function ( ms ) {
      select.appendChild( Dom.el( "option" , { text: Links.pauseText( ms ) , attrs: { value: String( ms ) } } ) );
    } );
  },

  async initGameSettings() {
    if ( this.settingsReady ) return;
    this.settingsReady = true;
    Dom.show( Dom.get( "game-settings-panel" ) , true );
    this.fillPauses( Dom.get( "setting-trick-pause" ) );
    [ "setting-join-signup" , "setting-observers-all" , "setting-table-chat" , "setting-global-chat" , "setting-trick-pause" ].forEach( function ( id ) {
      Dom.get( id ).addEventListener( "change" , Links.saveSettings.bind( Links ) );
    } );
    try {
      const settings = await Api.getSettings();
      Dom.get( "setting-join-signup" ).checked = settings.allow_join_signup;
      Dom.get( "setting-observers-all" ).checked = settings.observers_see_all;
      // Stored as "off" so older databases read as on; shown as "on".
      Dom.get( "setting-table-chat" ).checked = !settings.table_chat_off;
      Dom.get( "setting-global-chat" ).checked = !settings.global_chat_off;
      // 0 (never set) is the built-in pause, which is on the list.
      Dom.get( "setting-trick-pause" ).value = String( settings.trick_pause_ms || 1100 );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async initSignupLinks( me ) {
    if ( this.linksReady ) return;
    this.linksReady = true;
    this.showMaker = me.role === "admin";
    Dom.show( Dom.get( "signup-links-panel" ) , true );
    Dom.show( Dom.get( "signup-links-by-head" ) , this.showMaker );
    Dom.get( "signup-link-form" ).addEventListener( "submit" , this.onCreate.bind( this ) );
    Dom.get( "signup-link-copy" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "signup-link-value" ) , Dom.get( "signup-link-copied" ) );
    } );
    await this.refresh();
  },

  async saveSettings() {
    try {
      await Api.saveSettings( {
        allow_join_signup: Dom.get( "setting-join-signup" ).checked,
        observers_see_all: Dom.get( "setting-observers-all" ).checked,
        table_chat_off: !Dom.get( "setting-table-chat" ).checked,
        global_chat_off: !Dom.get( "setting-global-chat" ).checked,
        trick_pause_ms: Number( Dom.get( "setting-trick-pause" ).value ),
      } );
      Toast.show( I18n.get( "game_settings.saved" ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async onCreate( event ) {
    event.preventDefault();
    try {
      const created = await Api.createSignupLink(
        Dom.get( "signup-link-label" ).value.trim(),
        Number( Dom.get( "signup-link-uses" ).value ),
        Number( Dom.get( "signup-link-days" ).value ),
      );
      // Shown once: the server keeps only a hash of it.
      Dom.text( Dom.get( "signup-link-value" ) , window.location.origin + "/#/signup/" + created.credential );
      Dom.show( Dom.get( "signup-link-box" ) , true );
      Dom.get( "signup-link-label" ).value = "";
      await this.refresh();
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async refresh() {
    const body = Dom.get( "signup-links-table" ).querySelector( "tbody" );
    let links = [];
    try {
      links = await Api.listSignupLinks();
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    Dom.clear( body );
    links.forEach( function ( link ) {
      const state = link.revoked ? "signup_links.revoked" : ( link.live ? "signup_links.live" : "signup_links.expired" );
      const actions = Dom.el( "td" );
      if ( link.live ) {
        actions.appendChild( Dom.el( "button" , {
          class: "secondary small",
          text: I18n.get( "signup_links.revoke" ),
          attrs: { type: "button" },
          on: { click: async function () {
            try {
              await Api.revokeSignupLink( link.id );
              await Links.refresh();
            } catch ( error ) {
              Toast.error( error );
            }
          } },
        } ) );
      }
      body.appendChild( Dom.el( "tr" , { children: [
        Dom.el( "td" , { text: link.label } ),
        Links.showMaker ? Dom.el( "td" , { text: link.created_by_name || "—" } ) : null,
        Dom.el( "td" , { text: I18n.format( "signup_links.used_of" , { used: link.used , max: link.max_uses } ) } ),
        Dom.el( "td" , { text: I18n.get( state ) } ),
        actions,
      ].filter( Boolean ) } ) );
    } );
  },
};
