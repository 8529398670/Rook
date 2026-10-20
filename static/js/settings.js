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
    Dom.get( "device-link-button" ).addEventListener( "click" , this.onDeviceLink.bind( this ) );
    // Anyone may delete their own account -- except the owner, whom the
    // server keeps (see models/superadmin.go), so they are not offered it.
    Dom.show( Dom.get( "delete-me-row" ) , !me.owner && me.auth !== "api_key" );
    Dom.get( "delete-me-button" ).addEventListener( "click" , function () { Sheet.open( "sheet-confirm-delete-me" ); } );
    Dom.get( "delete-me-confirm" ).addEventListener( "click" , this.onDeleteMe.bind( this ) );
    Dom.get( "device-link-copy" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "device-link-value" ) , Dom.get( "device-link-copied" ) );
    } );
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
    if ( this.me.can_invite ) Links.initSignupLinks( this.me );

    // The Players card and the app-wide game settings are the owner's
    // alone, not every admin's.
    if ( !this.me.owner ) return;
    Dom.show( Dom.get( "admin-panel" ) , true );
    Links.initGameSettings();
    Dom.get( "create-user-form" ).addEventListener( "submit" , this.onCreateUser.bind( this ) );
    Dom.get( "copy-link-button" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "login-link-value" ) , Dom.get( "copied-notice" ) );
    } );
    Dom.get( "edit-user-form" ).addEventListener( "submit" , this.onEditName.bind( this ) );
    Dom.get( "delete-user-confirm" ).addEventListener( "click" , this.onDeleteUser.bind( this ) );
    Dom.get( "revoke-user-confirm" ).addEventListener( "click" , this.onRevokeUser.bind( this ) );
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

  // Deleting your own account. Afterwards there is nobody signed in, so the
  // page starts over at the front door.
  async onDeleteMe() {
    try {
      await Api.deleteMe();
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    window.location.href = "/";
  },

  // A fresh one-time link to yourself, for signing in on another device.
  // Each press mints a new one; earlier ones still work until used or
  // expired.
  async onDeviceLink() {
    try {
      const issued = await Api.myLoginLink();
      Dom.text( Dom.get( "device-link-value" ) , window.location.origin + issued.login_path );
      Dom.text( Dom.get( "device-link-notice" ) , I18n.format( "device_link.notice" , {
        hours: Math.max( 1 , Math.round( issued.expires_in_seconds / 3600 ) ),
      } ) );
      Dom.show( Dom.get( "device-link-box" ) , true );
    } catch ( error ) {
      Toast.error( error );
    }
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

      // Undo / redo, at the tables they sit at. Admins can always, so for
      // an admin it is shown on and disabled, like Can invite.
      const undoSwitch = Dom.el( "input" , {
        attrs: { type: "checkbox" , role: "switch" , "aria-label": I18n.format( "admin.can_undo_label" , { name: user.display_name } ) },
        on: { change: async function ( event ) {
          try {
            await Api.setCanUndo( user.id , event.target.checked );
          } catch ( error ) {
            event.target.checked = !event.target.checked;
            App.showError( error.message );
          }
        } },
      } );
      undoSwitch.checked = user.role === "admin" || user.can_undo === true;
      undoSwitch.disabled = user.role === "admin";

      const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
      Art.avatar( avatar , user );

      // Any admin may promote or demote anyone, themselves included --
      // except the superadmin, who always stays an admin.
      const roleSelect = Dom.el( "select" , {
        attrs: { "aria-label": I18n.format( "admin.role_label" , { name: user.display_name } ) },
        children: [ "user" , "admin" ].map( function ( role ) {
          return Dom.el( "option" , { text: I18n.get( "admin.role_" + role ) , attrs: { value: role } } );
        } ),
        on: { change: async function ( event ) {
          const role = event.target.value;
          try {
            await Api.setRole( user.id , role );
          } catch ( error ) {
            event.target.value = user.role;
            Toast.error( error );
            return;
          }
          // Demoting yourself: start over as a player, without the admin
          // screens this page has already drawn.
          if ( user.id === Settings.me.id && role !== "admin" ) {
            window.location.reload();
            return;
          }
          Toast.show( I18n.format( role === "admin" ? "admin.promoted" : "admin.demoted" , { name: user.display_name } ) );
          await Settings.refreshUsers();
        } },
      } );
      roleSelect.value = user.role;
      roleSelect.disabled = user.superadmin === true;
      const roleCell = [ roleSelect ];
      // Only the owner sees which account is the owner.
      if ( user.superadmin && Settings.me.owner ) roleCell.push( Dom.el( "span" , { class: "chip owner-chip" , text: I18n.get( "admin.superadmin_tag" ) , attrs: { title: I18n.get( "admin.superadmin_help" ) } } ) );

      // Built with createElement + textContent, never an HTML string: a
      // display name is user-controlled input.
      body.appendChild( Dom.el( "tr" , { children: [
        Dom.el( "td" , { children: [ avatar ] } ),
        Dom.el( "td" , { text: user.display_name } ),
        Dom.el( "td" , { children: [ Dom.el( "div" , { class: "role-cell" , children: roleCell } ) ] } ),
        Dom.el( "td" , { text: I18n.get( statusKey ) } ),
        Dom.el( "td" , { children: [ inviteSwitch ] } ),
        Dom.el( "td" , { children: [ hintSwitch ] } ),
        Dom.el( "td" , { children: [ undoSwitch ] } ),
        Dom.el( "td" , { children: [
          Dom.el( "div" , { class: "row-actions" , children: [
            action( "admin.reissue_button" , async function () {
              const issued = await Api.reissueLogin( user.id );
              Settings.showLoginLink( issued.login_path );
            } ),
            action( "admin.edit_button" , async function () { await Settings.openEditUser( user ); } ),
            // For "someone else may have this account's credentials": ends
            // every session, unused login link and API key, and leaves the
            // account enabled. Not on your own row -- the server refuses
            // that, since it would sign you out with no way back in.
            user.id === Settings.me.id ? null : Dom.el( "button" , {
              class: "secondary small",
              text: I18n.get( "admin.revoke_button" ),
              on: { click: function () { Settings.confirmRevokeUser( user ); } },
            } ),
            user.superadmin && !user.disabled ? null : action( user.disabled ? "admin.enable_button" : "admin.disable_button" , async function () {
              await Api.setDisabled( user.id , !user.disabled );
              await Settings.refreshUsers();
            } ),
            user.id === Settings.me.id || user.superadmin ? null : Dom.el( "button" , {
              class: "secondary small danger-text",
              text: I18n.get( "admin.delete_button" ),
              on: { click: function () { Settings.confirmDeleteUser( user ); } },
            } ),
          ].filter( Boolean ) } ),
        ] } ),
      ] } ) );
    } );
  },

  // The admin's "edit player" sheet: a name, and the same icon and color
  // picker everyone has for themselves. Choices save as they are made.
  editing: null,
  editPicker: null,

  async openEditUser( user ) {
    if ( !await Profile.loadOptions() ) return;
    this.editing = Object.assign( {} , user );
    Dom.text( Dom.get( "edit-user-heading" ) , user.display_name );
    Dom.get( "edit-user-name" ).value = user.display_name;
    this.editPicker = Profile.picker( Dom.get( "edit-user-icons" ) , Dom.get( "edit-user-colors" ) , {
      person: function () { return Settings.editing; },
      preview: function ( person ) { Art.avatar( Dom.get( "edit-user-preview" ) , person ); },
      save: async function ( icon , color ) {
        try {
          await Api.adminSetAvatar( Settings.editing.id , icon , color );
          Settings.editing.icon = icon;
          Settings.editing.color = color;
          Settings.editedSelf();
        } catch ( error ) {
          Toast.error( error );
        }
      },
    } );
    Sheet.open( "sheet-edit-user" , { onClose: function () { Settings.refreshUsers(); } } );
  },

  async onEditName( event ) {
    event.preventDefault();
    const name = Dom.get( "edit-user-name" ).value.trim();
    try {
      await Api.adminRename( this.editing.id , name );
      this.editing.display_name = name;
      Dom.text( Dom.get( "edit-user-heading" ) , name );
      this.editPicker.mark();
      this.editedSelf();
      Toast.show( I18n.get( "account.saved_notice" ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // An admin editing their own row: keep this page's idea of them in step.
  editedSelf() {
    if ( !this.editing || this.editing.id !== this.me.id ) return;
    this.me.display_name = this.editing.display_name;
    this.me.icon = this.editing.icon;
    this.me.color = this.editing.color;
    Dom.get( "display-name" ).value = this.me.display_name;
    this.renderWho();
    Profile.renderPreview();
    if ( Profile.mine ) Profile.mine.mark();
  },

  revoking: null,

  confirmRevokeUser( user ) {
    this.revoking = user;
    Dom.text( Dom.get( "revoke-user-body" ) , I18n.format( "admin.revoke_body" , { name: user.display_name } ) );
    Sheet.open( "sheet-confirm-revoke-user" );
  },

  async onRevokeUser() {
    const user = this.revoking;
    if ( !user ) return;
    try {
      await Api.revokeAccess( user.id );
      Sheet.close();
      Toast.show( I18n.format( "admin.revoked" , { name: user.display_name } ) );
      if ( Keys.me && Keys.me.api_keys_enabled !== false && Keys.me.owner ) await Keys.refreshAll();
    } catch ( error ) {
      Toast.error( error );
    }
    this.revoking = null;
  },

  deleting: null,

  confirmDeleteUser( user ) {
    this.deleting = user;
    Dom.text( Dom.get( "delete-user-body" ) , I18n.format( "admin.delete_body" , { name: user.display_name } ) );
    Sheet.open( "sheet-confirm-delete-user" );
  },

  async onDeleteUser() {
    const user = this.deleting;
    if ( !user ) return;
    try {
      await Api.deleteUser( user.id );
      Sheet.close();
      Toast.show( I18n.format( "admin.deleted" , { name: user.display_name } ) );
    } catch ( error ) {
      Toast.error( error );
    }
    this.deleting = null;
    await this.refreshUsers();
  },
};
