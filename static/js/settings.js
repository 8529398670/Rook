// The settings screen: your account, the light/dark choice, API keys, and --
// for admins -- who can get in. This is the template's account/admin page,
// moved off the front door now that the front door is the lobby.
const Settings = {
  screenId: "screen-settings",
  me: null,
  ready: false,

  init( me ) {
    this.me = me;
    Dom.get( "settings-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.get( "rename-form" ).addEventListener( "submit" , this.onRename.bind( this ) );
    Dom.get( "logout-button" ).addEventListener( "click" , this.onLogout.bind( this ) );
    Dom.all( "[data-theme-choice]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        Theme.set( button.getAttribute( "data-theme-choice" ) );
        Settings.renderTheme();
      } );
    } );
    this.renderWho();
    Zoom.bind( Dom.get( "settings-zoom" ) );
  },

  async enter() {
    Dom.get( "display-name" ).value = this.me.display_name;
    this.renderTheme();
    await Profile.enter();
    if ( this.ready ) return;
    this.ready = true;

    // Each panel owns its own file and its own init(); this is the one line
    // that knows the API keys screen exists.
    await Keys.init( this.me );

    if ( this.me.role !== "admin" ) return;
    Dom.show( Dom.get( "admin-panel" ) , true );
    Links.init();
    Dom.get( "create-user-form" ).addEventListener( "submit" , this.onCreateUser.bind( this ) );
    Dom.get( "copy-link-button" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "login-link-value" ) , Dom.get( "copied-notice" ) );
    } );
    await this.refreshUsers();
  },

  leave() {},

  renderWho() {
    const roleKey = this.me.role === "admin" ? "account.role_admin" : "account.role_user";
    Dom.text( Dom.get( "who-am-i" ) , I18n.format( "account.signed_in_as" , { name: this.me.display_name , role: I18n.get( roleKey ) } ) );
    Art.avatar( Dom.get( "open-settings" ) , this.me );
  },

  renderTheme() {
    const current = Theme.get();
    Dom.all( "[data-theme-choice]" ).forEach( function ( button ) {
      button.setAttribute( "aria-checked" , String( button.getAttribute( "data-theme-choice" ) === current ) );
    } );
  },

  async onRename( event ) {
    event.preventDefault();
    const value = Dom.get( "display-name" ).value.trim();
    try {
      await Api.rename( value );
      this.me.display_name = value;
      this.renderWho();
      Dom.flash( Dom.get( "rename-saved" ) );
    } catch ( error ) {
      App.showError( error.message );
    }
  },

  async onLogout() {
    try {
      await Api.logout();
    } catch ( error ) {
      // Even if the call failed, reloading lands on the signed-out page,
      // which is the honest state to show.
    }
    window.location.href = "/";
  },

  async onCreateUser( event ) {
    event.preventDefault();
    const name = Dom.get( "new-name" ).value.trim();
    const role = Dom.get( "new-role" ).value;
    try {
      const created = await Api.createUser( name , role );
      this.showLoginLink( created.login_path );
      Dom.get( "new-name" ).value = "";
      await this.refreshUsers();
    } catch ( error ) {
      App.showError( error.message );
    }
  },

  // The server returns a path, not a full URL: behind a reverse proxy it does
  // not reliably know its own public origin, whereas the browser always does.
  showLoginLink( loginPath ) {
    Dom.text( Dom.get( "login-link-value" ) , window.location.origin + loginPath );
    Dom.show( Dom.get( "login-link-box" ) , true );
  },

  async refreshUsers() {
    const body = Dom.get( "users-table" ).querySelector( "tbody" );
    let users = [];
    try {
      users = await Api.listUsers();
    } catch ( error ) {
      App.showError( error.message );
      return;
    }

    Dom.clear( body );
    users.forEach( function ( user ) {
      const statusKey = user.disabled ? "admin.status_disabled" : "admin.status_active";
      const action = function ( textKey , run ) {
        return Dom.el( "button" , {
          class: "secondary small",
          text: I18n.get( textKey ),
          on: { click: async function () {
            try {
              await run();
            } catch ( error ) {
              App.showError( error.message );
            }
          } },
        } );
      };

      // Admins can always invite, so the switch only means something on a
      // plain user -- for an admin it is shown on and disabled.
      const inviteSwitch = Dom.el( "input" , {
        attrs: { type: "checkbox" , role: "switch" , "aria-label": I18n.format( "admin.can_invite_label" , { name: user.display_name } ) },
        on: { change: async function ( event ) {
          try {
            await Api.setCanInvite( user.id , event.target.checked );
          } catch ( error ) {
            event.target.checked = !event.target.checked;
            App.showError( error.message );
          }
        } },
      } );
      inviteSwitch.checked = user.role === "admin" || user.can_invite === true;
      inviteSwitch.disabled = user.role === "admin";

      // Hints are per person, admins included: an admin may want them off.
      const hintSwitch = Dom.el( "input" , {
        attrs: { type: "checkbox" , role: "switch" , "aria-label": I18n.format( "admin.can_hint_label" , { name: user.display_name } ) },
        on: { change: async function ( event ) {
          try {
            await Api.setCanHint( user.id , event.target.checked );
          } catch ( error ) {
            event.target.checked = !event.target.checked;
            App.showError( error.message );
          }
        } },
      } );
      hintSwitch.checked = user.can_hint === true;

      // Built with createElement + textContent, never an HTML string: a
      // display name is user-controlled input.
      body.appendChild( Dom.el( "tr" , { children: [
        Dom.el( "td" , { text: user.display_name } ),
        Dom.el( "td" , { text: user.role } ),
        Dom.el( "td" , { text: I18n.get( statusKey ) } ),
        Dom.el( "td" , { children: [ inviteSwitch ] } ),
        Dom.el( "td" , { children: [ hintSwitch ] } ),
        Dom.el( "td" , { children: [
          Dom.el( "div" , { class: "row-actions" , children: [
            action( "admin.reissue_button" , async function () {
              const issued = await Api.reissueLogin( user.id );
              Settings.showLoginLink( issued.login_path );
            } ),
            action( user.disabled ? "admin.enable_button" : "admin.disable_button" , async function () {
              await Api.setDisabled( user.id , !user.disabled );
              await Settings.refreshUsers();
            } ),
          ] } ),
        ] } ),
      ] } ) );
    } );
  },
};
