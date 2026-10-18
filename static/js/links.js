// Admin-only Settings panels about how people get in and what watchers see:
// the app-wide game settings, and multi-use sign-up links.
const Links = {
  ready: false,

  async init() {
    if ( this.ready ) return;
    this.ready = true;
    Dom.show( Dom.get( "game-settings-panel" ) , true );
    Dom.show( Dom.get( "signup-links-panel" ) , true );

    [ "setting-join-signup" , "setting-observers-all" ].forEach( function ( id ) {
      Dom.get( id ).addEventListener( "change" , Links.saveSettings.bind( Links ) );
    } );
    Dom.get( "signup-link-form" ).addEventListener( "submit" , this.onCreate.bind( this ) );
    Dom.get( "signup-link-copy" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "signup-link-value" ) , Dom.get( "signup-link-copied" ) );
    } );

    try {
      const settings = await Api.getSettings();
      Dom.get( "setting-join-signup" ).checked = settings.allow_join_signup;
      Dom.get( "setting-observers-all" ).checked = settings.observers_see_all;
    } catch ( error ) {
      Toast.error( error );
    }
    await this.refresh();
  },

  async saveSettings() {
    try {
      await Api.saveSettings( {
        allow_join_signup: Dom.get( "setting-join-signup" ).checked,
        observers_see_all: Dom.get( "setting-observers-all" ).checked,
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
        Dom.el( "td" , { text: I18n.format( "signup_links.used_of" , { used: link.used , max: link.max_uses } ) } ),
        Dom.el( "td" , { text: I18n.get( state ) } ),
        actions,
      ] } ) );
    } );
  },
};
