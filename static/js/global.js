// The global chat -- one conversation for everyone signed in, at any table
// or none -- and the "online now" list. Its stream opens once, at sign-in,
// and stays open on every screen: having it open is what puts you on the
// online list. It is drawn in three places: the lobby's side panel, a dock
// beside your cards, and the "Everyone" tab of a table's chat sheet. Lines
// are plain text from other players and only ever set with textContent.
const Global = {
  me: null,
  view: null,
  stream: null,
  seen: 0,
  primed: false,
  enabled: null,

  start( me ) {
    this.me = me;
    [ [ "lobby-global-form" , "lobby-global-input" ] , [ "global-dock-form" , "global-dock-input" ] , [ "global-form" , "global-input" ] ].forEach( function ( pair ) {
      Dom.get( pair[ 0 ] ).addEventListener( "submit" , function ( event ) {
        event.preventDefault();
        Global.send( Dom.get( pair[ 1 ] ) );
      } );
    } );
    this.stream = Stream.open( "/api/global/events" , {
      global: function ( view ) { Global.update( view ); },
    } );
  },

  // Whether the global chat shows at all: an admin may have switched it
  // off, and each player may hide it on their device (chat.js).
  available() {
    return Boolean( this.me && this.view && this.view.enabled );
  },

  shown() {
    return this.available() && Chat.shown( "global" );
  },

  latest() {
    const chat = this.view ? this.view.chat : [];
    return chat.length ? chat[ chat.length - 1 ].seq : 0;
  },

  unread() {
    return this.shown() && this.latest() > this.seen;
  },

  update( view ) {
    this.view = view;
    if ( !this.primed ) {
      // Signing in should not light up everything said before you arrived.
      this.seen = this.latest();
      this.primed = true;
    }
    if ( this.enabled !== view.enabled ) {
      // Switched on or off by an admin: the docks and the sheet's tabs
      // change with it.
      this.enabled = view.enabled;
      Chat.refresh();
      return;
    }
    this.render();
  },

  async send( input ) {
    const text = input.value.trim();
    if ( !text ) return;
    try {
      const view = await Api.globalChat( text );
      input.value = "";
      this.update( view );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // Draw every place the global chat shows, and mark lines read where
  // someone can see them.
  render() {
    if ( !this.view ) return;
    this.renderLobby();
    if ( Chat.dockedSide( "global" ) !== "off" ) {
      this.fill( Dom.get( "global-dock-list" ) );
      if ( App.screen === Table ) this.seen = this.latest();
    }
    if ( Sheet.isOpen( "sheet-chat" ) && Chat.tab === "global" ) {
      this.fill( Dom.get( "global-list" ) );
      this.seen = this.latest();
    }
    Chat.showUnread();
  },

  renderLobby() {
    const online = this.view.online;
    Dom.text( Dom.get( "online-heading" ) , I18n.format( "global.online_heading" , { count: online.length } ) );
    const list = Dom.get( "online-list" );
    Dom.clear( list );
    online.forEach( function ( person ) {
      const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
      Art.avatar( avatar , person );
      list.appendChild( Dom.el( "li" , {
        class: "online-person" + ( Global.me && person.id === Global.me.id ? " is-you" : "" ),
        children: [ avatar , Dom.el( "span" , { class: "online-name" , text: person.display_name } ) ],
      } ) );
    } );

    // Hidden by this player: no panel. Switched off by an admin: say so,
    // rather than leaving an empty box.
    Dom.show( Dom.get( "lobby-global" ) , Chat.shown( "global" ) );
    Dom.show( Dom.get( "lobby-global-off" ) , !this.view.enabled );
    Dom.show( Dom.get( "lobby-global-list" ) , this.view.enabled );
    Dom.show( Dom.get( "lobby-global-form" ) , this.view.enabled );
    if ( this.view.enabled ) {
      this.fill( Dom.get( "lobby-global-list" ) );
      if ( App.screen === Lobby && Chat.shown( "global" ) ) this.seen = this.latest();
    }
  },

  // Fill one list with the recent lines. Redrawn only when something in it
  // changed, so it does not jump while you scroll back through it.
  fill( list ) {
    const chat = this.view.chat;
    const shown = this.latest() + "|" + Chat.digest( chat );
    if ( list.getAttribute( "data-shown" ) === shown || Chat.busy( list ) ) return;
    list.setAttribute( "data-shown" , shown );
    Dom.clear( list );
    if ( chat.length === 0 ) list.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "global.empty" ) } ) );
    chat.forEach( function ( line ) { list.appendChild( Global.lineEl( line ) ); } );
    list.scrollTop = list.scrollHeight;
  },

  lineEl( line ) {
    const mine = Boolean( this.me && line.user_id === this.me.id );
    const item = Dom.el( "li" , { class: "chat-line" + ( mine ? " is-mine" : "" ) , children: [
      Dom.el( "span" , { class: "chat-name" , text: line.name } ),
      Chat.textEl( line ),
    ] } );
    // Your own lines can be edited or deleted, the same way as at a table;
    // the owner may edit or delete anyone's.
    if ( mine || ( this.me && this.me.owner ) ) {
      item.appendChild( Chat.ownActions( item , line , {
        save: async function ( seq , text ) { Global.update( await Api.editGlobalChat( seq , text ) ); },
        remove: async function ( seq ) { Global.update( await Api.deleteGlobalChat( seq ) ); },
        redraw: function () { Global.redraw(); },
      } ) );
    }
    return item;
  },

  // Draw every global list afresh, after an edit or delete.
  redraw() {
    Dom.all( "#lobby-global-list, #global-dock-list, #global-list" ).forEach( function ( list ) { list.removeAttribute( "data-shown" ); } );
    this.render();
  },
};
