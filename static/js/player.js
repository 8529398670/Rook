// The player menu: tap someone's name -- in "Online now", or on a line of
// the chat -- to message them, or to invite them to play beside you
// (partner) or against you (opponent). An invite holds a seat at the
// waiting table you are sitting at, when you may hand out its seats;
// otherwise it opens a new table for the two of you.
const Player = {
  person: null,
  // The seat this menu was opened from, when you may manage it; else -1.
  seat: -1,

  init() {
    this.watchActivity();
    Dom.get( "player-sheet-message" ).addEventListener( "click" , function () {
      const person = Player.person;
      Sheet.close();
      if ( person ) Global.message( person );
    } );
    Dom.get( "player-sheet-partner" ).addEventListener( "click" , function () { Player.invite( "partner" ); } );
    Dom.get( "player-sheet-opponent" ).addEventListener( "click" , function () { Player.invite( "opponent" ); } );
    Dom.get( "player-sheet-manage" ).addEventListener( "click" , function () {
      const seat = Player.seat;
      Sheet.close();
      if ( seat >= 0 ) Table.manageSeat( seat );
    } );
  },

  // A name on a chat line that opens the menu.
  nameButton( person , text ) {
    return Dom.el( "button" , {
      class: "chat-name chat-name-button",
      text: text,
      attrs: { type: "button" , "aria-label": I18n.format( "player.open_label" , { name: text } ) },
      on: { click: function () { Player.open( person ); } },
    } );
  },

  // Someone on the online list, if they are on it: their current name and
  // avatar.
  online( id ) {
    const online = ( Global.view && Global.view.online ) || [];
    return online.find( function ( person ) { return person.id === id; } ) || null;
  },

  // seat: the seat at the table on screen it was opened from, when you may
  // manage that seat; leave it out otherwise.
  open( person , seat ) {
    if ( !person || !Global.me || person.id === Global.me.id ) return;
    const live = this.online( person.id );
    this.person = live || person;
    this.seat = typeof seat === "number" ? seat : -1;
    Art.avatar( Dom.get( "player-sheet-avatar" ) , this.person );
    Dom.text( Dom.get( "player-sheet-name" ) , this.person.display_name );
    Dom.text( Dom.get( "player-sheet-status" ) , I18n.get( live ? "player.online" : "player.offline" ) );
    if ( live ) this.showPresence( this.person.id );
    const playing = this.playing();
    Dom.show( Dom.get( "player-sheet-partner" ) , !playing );
    Dom.show( Dom.get( "player-sheet-opponent" ) , !playing );
    Dom.show( Dom.get( "player-sheet-manage" ) , this.seat >= 0 );
    const here = this.hereTable();
    Dom.text( Dom.get( "player-sheet-where" ) , playing ? I18n.get( "player.where_playing" ) :
      here ? I18n.format( "player.where_here" , { table: Table.view.name } ) : I18n.get( "player.where_new" ) );
    Sheet.open( "sheet-player" );
  },

  // Asks the server whether they are idle -- nothing done in the app for
  // five minutes -- and since when, and puts that under their name.
  async showPresence( id ) {
    let presence;
    try {
      presence = await Api.presence( id );
    } catch ( error ) {
      return;
    }
    if ( !this.person || this.person.id !== id ) return;
    const status = Dom.get( "player-sheet-status" );
    if ( !presence.online ) {
      Dom.text( status , I18n.get( "player.offline" ) );
      return;
    }
    if ( !presence.idle ) {
      Dom.text( status , I18n.get( "player.online" ) );
      return;
    }
    const seconds = presence.idle_seconds || 0;
    // From the server's count rather than its timestamp, so a page whose
    // clock is off still shows the right time.
    const since = new Date( Date.now() - seconds * 1000 );
    Dom.text( status , I18n.format( "player.idle" , { since: this.when( since ) , for: this.howLong( seconds ) } ) );
  },

  // A time today, or a date and time before that.
  when( date ) {
    const today = date.toDateString() === new Date().toDateString();
    return today ? date.toLocaleTimeString( undefined , { timeStyle: "short" } ) :
      date.toLocaleString( undefined , { dateStyle: "medium" , timeStyle: "short" } );
  },

  howLong( seconds ) {
    const minutes = Math.floor( seconds / 60 );
    const hours = Math.floor( minutes / 60 );
    const days = Math.floor( hours / 24 );
    if ( days > 0 ) return I18n.format( "player.idle_days" , { d: days , h: hours % 24 } );
    if ( hours > 0 ) return I18n.format( "player.idle_hours" , { h: hours , m: minutes % 60 } );
    return I18n.format( "player.idle_minutes" , { m: minutes } );
  },

  // Tells the server you are using the app -- a click, tap, key, scroll or
  // coming back to the tab -- at most once a minute, so you show as idle
  // only after five minutes of none of those (server/tables/activity.go).
  watchActivity() {
    let sent = Date.now();
    const report = function () {
      if ( document.visibilityState === "hidden" || Date.now() - sent < 60000 ) return;
      sent = Date.now();
      Api.activity().catch( function () {} );
    };
    [ "pointerdown" , "keydown" , "wheel" , "touchstart" , "scroll" ].forEach( function ( type ) {
      window.addEventListener( type , report , { passive: true , capture: true } );
    } );
    document.addEventListener( "visibilitychange" , report );
  },

  // Whether you are sitting in a game that has started at the table on
  // screen: an invite would open a new table and pull you away from it.
  playing() {
    const view = App.screen === Table ? Table.view : null;
    return Boolean( view ) && view.you >= 0 && view.status !== "waiting" && view.status !== "over";
  },

  // The table an invite goes to: the one on screen, if you sit there, the
  // game has not started, and you may hand out its seats.
  hereTable() {
    const view = App.screen === Table ? Table.view : null;
    if ( !view || view.status !== "waiting" || view.you < 0 ) return "";
    const me = Global.me || {};
    const host = view.seats[ view.you ] && view.seats[ view.you ].host;
    return host || me.can_invite ? Table.id : "";
  },

  async invite( as ) {
    const person = this.person;
    if ( !person ) return;
    const here = this.hereTable();
    const name = I18n.format( "player.table_name" , { a: Global.me.display_name , b: person.display_name } );
    try {
      const result = await Api.invitePlayer( person.id , as , here , name );
      Sheet.close();
      Toast.show( I18n.format( as === "partner" ? "player.invited_partner" : "player.invited_opponent" , { name: person.display_name } ) );
      if ( !here ) App.go( "#/t/" + result.table_id );
    } catch ( error ) {
      Toast.error( error );
    }
  },
};
