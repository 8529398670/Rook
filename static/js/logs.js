// The activity log, #/logs: the owner's record of every request, sign-in and
// admin action (server/logs). One day at a time, newest first, with the same
// search the server understands (kind:auth, status:5xx, user:ana, ...). Live
// listens on /api/owner/logs/stream, which only says "something new", and
// reloads the newest entries; nothing is polled.
const Logs = {
  screenId: "screen-logs",
  ready: false,
  day: "",
  next: null,
  live: null,
  searchTimer: null,

  init() {
    Dom.get( "logs-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.get( "logs-search-form" ).addEventListener( "submit" , ( event ) => {
      event.preventDefault();
      this.loadDays();
    } );
    Dom.get( "logs-search" ).addEventListener( "input" , () => {
      window.clearTimeout( this.searchTimer );
      this.searchTimer = window.setTimeout( () => this.loadDays() , 350 );
    } );
    Dom.get( "logs-day" ).addEventListener( "change" , ( event ) => {
      this.day = event.target.value;
      this.loadDay( false );
    } );
    Dom.get( "logs-more" ).addEventListener( "click" , () => this.loadDay( true ) );
    Dom.get( "logs-live" ).addEventListener( "change" , ( event ) => {
      if ( event.target.checked ) this.startLive(); else this.stopLive();
    } );
  },

  async enter() {
    if ( !this.ready ) {
      this.ready = true;
      this.init();
    }
    await this.loadDays();
    if ( Dom.get( "logs-live" ).checked ) this.startLive();
  },

  leave() {
    this.stopLive();
  },

  query() {
    return Dom.get( "logs-search" ).value.trim();
  },

  async loadDays() {
    let page;
    try {
      page = await Api.logDays( this.query() );
    } catch ( error ) {
      // 404: the activity log is off (logs.enabled: false in config.yaml).
      if ( error.status === 404 ) {
        Dom.show( Dom.get( "logs-off" ) , true );
        Dom.show( Dom.get( "logs-body" ) , false );
        return;
      }
      Toast.error( error );
      return;
    }
    Dom.show( Dom.get( "logs-off" ) , false );
    Dom.show( Dom.get( "logs-body" ) , true );
    this.renderSettings( page );
    const select = Dom.get( "logs-day" );
    Dom.clear( select );
    const days = ( page.days || [] ).slice().sort( function ( a , b ) { return a.date < b.date ? 1 : -1; } );
    days.forEach( ( day ) => {
      const count = day.matches !== undefined && day.matches !== null ? day.matches : day.entries;
      const key = day.matches !== undefined && day.matches !== null ? "logs.day_matches" : "logs.day_button";
      select.appendChild( Dom.el( "option" , { text: I18n.format( key , { date: day.date , count: count } ) , attrs: { value: day.date } } ) );
    } );
    if ( !days.some( ( day ) => day.date === this.day ) ) this.day = days.length > 0 ? days[ 0 ].date : "";
    select.value = this.day;
    await this.loadDay( false );
  },

  renderSettings( page ) {
    const settings = page.settings || {};
    const used = Math.round( ( page.total_bytes || 0 ) / 1024 / 1024 * 10 ) / 10;
    let line = I18n.format( "logs.settings_line" , {
      days: settings.retention_days,
      max: settings.max_total_mb + " MB",
      used: used + " MB",
      gzip: settings.compress_after_days,
    } );
    line += " " + I18n.get( settings.encrypted ? "logs.encrypted" : "logs.not_encrypted" );
    Dom.text( Dom.get( "logs-settings" ) , line );
  },

  async loadDay( more ) {
    const list = Dom.get( "logs-list" );
    const download = Dom.get( "logs-download" );
    if ( !more ) {
      Dom.clear( list );
      this.next = null;
    }
    Dom.show( download , this.day !== "" );
    if ( this.day === "" ) {
      Dom.show( Dom.get( "logs-empty" ) , true );
      Dom.show( Dom.get( "logs-more" ) , false );
      return;
    }
    download.href = "/api/owner/logs/days/" + encodeURIComponent( this.day ) + "/download" + ( this.query() ? "?q=" + encodeURIComponent( this.query() ) : "" );
    let page;
    try {
      page = await Api.logDay( this.day , this.query() , more ? this.next : null );
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    ( page.entries || [] ).forEach( ( entry ) => list.appendChild( this.row( entry ) ) );
    this.next = page.next === undefined ? null : page.next;
    Dom.show( Dom.get( "logs-empty" ) , list.children.length === 0 );
    Dom.show( Dom.get( "logs-more" ) , this.next !== null && this.next >= 0 );
  },

  // One line: when, what kind, who, and what happened.
  row( entry ) {
    const when = new Date( entry.ts );
    const time = Number.isNaN( when.getTime() ) ? "" : when.toLocaleTimeString( undefined , { hour: "2-digit" , minute: "2-digit" , second: "2-digit" } );
    const who = entry.actor && entry.actor.name ? entry.actor.name : I18n.get( "logs.anonymous" );
    let what = entry.msg || "";
    if ( entry.request ) {
      what = entry.request.method + " " + entry.request.path + " → " + entry.request.status + " (" + Math.round( entry.request.ms ) + " ms)";
    }
    const where = entry.ip_name ? entry.ip_name + " · " + entry.ip : ( entry.ip || "" );
    const kindLabel = I18n.get( "logs.kind_" + entry.kind ) || entry.kind;
    const level = entry.level === "error" || ( entry.request && entry.request.status >= 500 ) ? "error"
      : ( entry.level === "warn" || ( entry.request && entry.request.status >= 400 ) ? "warn" : "info" );
    return Dom.el( "li" , { class: "log-row log-" + level , children: [
      Dom.el( "div" , { class: "log-head" , children: [
        Dom.el( "time" , { class: "log-time" , text: time , attrs: { datetime: entry.ts } } ),
        Dom.el( "span" , { class: "log-kind log-kind-" + entry.kind , text: kindLabel } ),
        Dom.el( "span" , { class: "log-who" , text: who } ),
      ] } ),
      Dom.el( "div" , { class: "log-what" , text: what } ),
      where ? Dom.el( "div" , { class: "muted log-where" , text: where } ) : null,
    ].filter( Boolean ) } );
  },

  // Live: the stream says "changed" when more has been written; the newest
  // entries of the day showing are read again. Only today grows.
  startLive() {
    this.stopLive();
    try {
      this.live = new EventSource( "/api/owner/logs/stream" );
    } catch ( error ) {
      return;
    }
    this.live.addEventListener( "changed" , () => {
      const today = new Date().toISOString().slice( 0 , 10 );
      if ( this.day === today || this.day === "" ) this.loadDays();
    } );
    this.live.onerror = () => {
      // The browser retries by itself; a closed stream means signed out or
      // full, and Live is switched off so it says so.
      if ( this.live && this.live.readyState === EventSource.CLOSED ) {
        this.stopLive();
        Dom.get( "logs-live" ).checked = false;
        Toast.show( I18n.get( "logs.live_lost" ) , "error" );
      }
    };
  },

  stopLive() {
    if ( this.live ) this.live.close();
    this.live = null;
  },
};
