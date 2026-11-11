// The Logs screen, #/logs: the owner's record of every request, sign-in and
// admin action (server/logs). Laid out like the go-webserver skill's Logs
// screen: a search bar with a calendar button and Live/Prune, kind filters,
// names for addresses, and one day at a time as a table whose rows open to
// show the whole entry.
//
// The search box goes to the server as typed. Free words match anywhere;
// key:value narrows (kind:auth status:5xx user:ana path:/api ip:10.0.), a
// comma means "or" and a leading minus excludes. The kind buttons add a kind:
// term for you.
//
// Live listens on /api/owner/logs/stream, which only says "something new";
// nothing is polled. The stream is open only while this screen shows.
const Logs = {
  screenId: "screen-logs",
  ready: false,
  kinds: [ "request" , "ui" , "auth" , "admin" , "error" , "system" ],
  selected: new Set(),
  searchDelayMs: 300,
  searchTimer: null,
  // The least time between reads while Live is on, however busy the server is.
  liveMs: 2000,
  liveOn: false,
  liveSource: null,
  liveTimer: null,
  liveLast: 0,
  // The day shown and the month the calendar shows, as UTC dates like the
  // files' names. followToday stays true until someone picks a day, so the
  // day shown moves on at midnight UTC.
  shown: "",
  month: "",
  followToday: true,
  days: new Map(),
  dayTurn: 0,
  visible: false,

  init() {
    Dom.get( "logs-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.get( "logs-search-form" ).addEventListener( "submit" , ( event ) => {
      event.preventDefault();
      this.search();
    } );
    Dom.get( "logs-search" ).addEventListener( "input" , () => {
      window.clearTimeout( this.searchTimer );
      this.searchTimer = window.setTimeout( () => this.search() , this.searchDelayMs );
    } );
    Dom.get( "logs-live" ).addEventListener( "click" , () => this.setLive( !this.liveOn ) );
    Dom.get( "logs-prune" ).addEventListener( "click" , () => this.onPrune() );
    Dom.get( "ip-names-form" ).addEventListener( "submit" , ( event ) => {
      event.preventDefault();
      this.onSaveName();
    } );

    // The calendar is a popup under the date button. Clicks inside it (the
    // month arrows) leave it open; a picked day, Escape or a click elsewhere
    // closes it.
    Dom.get( "logs-date-button" ).addEventListener( "click" , () => this.toggleCalendar() );
    document.addEventListener( "click" , ( event ) => {
      if ( !Dom.get( "logs-date-picker" ).contains( event.target ) ) this.toggleCalendar( false );
    } );
    Dom.get( "logs-date-picker" ).addEventListener( "keydown" , ( event ) => {
      if ( event.key !== "Escape" || Dom.get( "logs-cal-menu" ).hidden ) return;
      event.stopPropagation();
      this.toggleCalendar( false );
      Dom.get( "logs-date-button" ).focus();
    } );
    Dom.get( "logs-cal-prev" ).addEventListener( "click" , () => this.shiftMonth( -1 ) );
    Dom.get( "logs-cal-next" ).addEventListener( "click" , () => this.shiftMonth( 1 ) );
    Dom.get( "logs-cal-today" ).addEventListener( "click" , () => this.pick( this.today() , true ) );

    this.shown = this.today();
    this.month = this.shown.slice( 0 , 7 );
    this.renderKinds();
  },

  async enter() {
    if ( !this.ready ) {
      this.ready = true;
      this.init();
    }
    this.visible = true;
    await Promise.all( [ this.refresh() , this.refreshNames() ] );
    this.followScreen();
  },

  leave() {
    this.visible = false;
    this.toggleCalendar( false );
    this.followScreen();
  },

  // ------------------------------------------------------------ search

  search() {
    return this.refresh( true );
  },

  query() {
    const typed = Dom.get( "logs-search" ).value.trim();
    const kinds = Array.from( this.selected );
    return [ typed , kinds.length ? "kind:" + kinds.join( "," ) : "" ].filter( Boolean ).join( " " );
  },

  // Made once: a press flips its own button, so the button under the pointer
  // is never taken away and put back.
  renderKinds() {
    const holder = Dom.get( "logs-kinds" );
    Dom.clear( holder );
    this.kinds.forEach( ( kind ) => {
      holder.appendChild( Dom.el( "button" , {
        class: "small logs-kind",
        text: I18n.get( "logs.kind_" + kind ) || kind,
        attrs: { type: "button" , "data-kind": kind },
        on: { click: () => {
          if ( this.selected.has( kind ) ) this.selected.delete( kind );
          else this.selected.add( kind );
          this.paintKinds();
          this.search();
        } },
      } ) );
    } );
    this.paintKinds();
  },

  paintKinds() {
    Array.from( Dom.get( "logs-kinds" ).children ).forEach( ( button ) => {
      button.setAttribute( "aria-pressed" , String( this.selected.has( button.getAttribute( "data-kind" ) ) ) );
    } );
  },

  // A new search moves to the newest matching day, unless the day shown has
  // matches of its own.
  async refresh( jump ) {
    let listing;
    try {
      listing = await Api.logDays( this.query() );
    } catch ( error ) {
      // 404: logs are off (logs.enabled: false in config.yaml).
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
    this.renderSettings( listing );
    const days = ( listing.days || [] ).slice().sort( function ( a , b ) { return a.date < b.date ? 1 : -1; } );
    this.days = new Map( days.map( ( day ) => [ day.date , day ] ) );

    const today = this.today();
    if ( this.followToday && this.shown !== today ) {
      this.shown = today;
      this.month = today.slice( 0 , 7 );
    }
    const searching = this.query() !== "";
    if ( jump && searching && days.length > 0 && !this.days.has( this.shown ) ) {
      this.shown = days[ 0 ].date;
      this.month = this.shown.slice( 0 , 7 );
      this.followToday = this.shown === today;
    }

    const nothing = searching && days.length === 0;
    const empty = Dom.get( "logs-empty" );
    Dom.text( empty , I18n.get( "logs.no_matches" ) );
    Dom.show( empty , nothing );
    Dom.show( Dom.get( "logs-day" ) , !nothing );
    this.renderCalendar();
    await this.renderDay();
  },

  renderSettings( listing ) {
    const settings = listing.settings || {};
    let line = I18n.format( "logs.settings_line" , {
      days: settings.retention_days,
      used: this.size( listing.total_bytes ),
      max: settings.max_total_mb + " MB",
      gzip: settings.compress_after_days,
    } );
    const stored = I18n.get( settings.encrypted ? "logs.encrypted" : "logs.not_encrypted" );
    if ( stored ) line += " · " + stored;
    if ( listing.dropped > 0 ) line += " · " + I18n.format( "logs.dropped" , { count: listing.dropped } );
    Dom.text( Dom.get( "logs-settings" ) , line );
  },

  // ---------------------------------------------------------- calendar

  toggleCalendar( open ) {
    const menu = Dom.get( "logs-cal-menu" );
    const show = open === undefined ? menu.hidden : open;
    if ( show === !menu.hidden ) return;
    menu.hidden = !show;
    Dom.get( "logs-date-button" ).setAttribute( "aria-expanded" , String( show ) );
    if ( show ) {
      this.month = this.shown.slice( 0 , 7 );
      this.renderCalendar();
    }
  },

  // Shows date. focus is for a pick made on the calendar: the popup closes
  // and the keyboard goes back to its button, which now names the new day.
  async pick( date , focus ) {
    this.shown = date;
    this.month = date.slice( 0 , 7 );
    this.followToday = date === this.today();
    Dom.show( Dom.get( "logs-day" ) , true );
    this.renderCalendar();
    if ( focus ) {
      this.toggleCalendar( false );
      Dom.get( "logs-date-button" ).focus( { preventScroll: true } );
    }
    await this.renderDay();
  },

  shiftMonth( step ) {
    const parts = this.month.split( "-" ).map( Number );
    this.month = new Date( Date.UTC( parts[ 0 ] , parts[ 1 ] - 1 + step , 1 ) ).toISOString().slice( 0 , 7 );
    this.renderCalendar();
  },

  // A month of buttons, one per date. A date with entries (or matches, while
  // searching) can be picked, and so can today; the rest are disabled.
  renderCalendar() {
    const today = this.today();
    const searching = this.query() !== "";
    const dates = Array.from( this.days.keys() ).sort();
    const oldest = dates.length ? dates[ 0 ].slice( 0 , 7 ) : today.slice( 0 , 7 );
    const newest = [ today.slice( 0 , 7 ) , dates.length ? dates[ dates.length - 1 ].slice( 0 , 7 ) : "" ].sort()[ 1 ];
    const parts = this.month.split( "-" ).map( Number );
    const first = new Date( Date.UTC( parts[ 0 ] , parts[ 1 ] - 1 , 1 ) );
    const length = new Date( Date.UTC( parts[ 0 ] , parts[ 1 ] , 0 ) ).getUTCDate();

    Dom.text( Dom.get( "logs-date-label" ) , this.dayLabel( this.shown ) );
    Dom.text( Dom.get( "logs-cal-title" ) , first.toLocaleDateString( undefined , { month: "long" , year: "numeric" , timeZone: "UTC" } ) );
    Dom.get( "logs-cal-prev" ).disabled = this.month <= oldest;
    Dom.get( "logs-cal-next" ).disabled = this.month >= newest;

    const grid = Dom.get( "logs-cal-grid" );
    Dom.clear( grid );
    // 2026-01-04 was a Sunday; the week starts there.
    for ( let weekday = 0 ; weekday < 7 ; weekday += 1 ) {
      const name = new Date( Date.UTC( 2026 , 0 , 4 + weekday ) ).toLocaleDateString( undefined , { weekday: "narrow" , timeZone: "UTC" } );
      grid.appendChild( Dom.el( "span" , { class: "log-cal-weekday" , text: name , attrs: { "aria-hidden": "true" } } ) );
    }
    for ( let blank = 0 ; blank < first.getUTCDay() ; blank += 1 ) grid.appendChild( Dom.el( "span" ) );
    for ( let number = 1 ; number <= length ; number += 1 ) {
      const date = this.month + "-" + String( number ).padStart( 2 , "0" );
      const day = this.days.get( date );
      const count = day ? ( searching ? day.matches : day.entries ) : 0;
      const label = I18n.format( searching ? "logs.day_matches" : "logs.day_button" , {
        date: this.dayLabel( date ),
        count: Number( count || 0 ).toLocaleString(),
      } );
      const cell = Dom.el( "button" , {
        class: "log-cal-day" + ( day ? " has-logs" : "" ) + ( date === today ? " is-today" : "" ),
        attrs: { type: "button" , "aria-pressed": String( date === this.shown ) , "aria-label": label , title: label },
        children: [
          Dom.el( "span" , { text: String( number ) } ),
          day && day.errors > 0 ? Dom.el( "span" , { class: "log-cal-dot" , attrs: { "aria-hidden": "true" } } ) : null,
        ].filter( Boolean ),
        on: { click: () => this.pick( date , true ) },
      } );
      cell.disabled = !day && date !== today;
      grid.appendChild( cell );
    }
  },

  // --------------------------------------------------------------- day

  // The day shown, folded or not as it was if it is the same day. The new day
  // is built and its first page read before it replaces the old one, so the
  // page never shrinks to nothing while it waits. Only the latest call lands.
  async renderDay() {
    const holder = Dom.get( "logs-day" );
    const old = holder.querySelector( "details.log-day" );
    const open = old && old.getAttribute( "data-date" ) === this.shown ? old.open : true;
    const searching = this.query() !== "";
    const day = this.days.get( this.shown ) || {
      date: this.shown, bytes: 0, entries: 0, errors: 0, compressed: false,
      matches: searching ? 0 : undefined, missing: true,
    };
    const turn = this.dayTurn = this.dayTurn + 1;
    const block = await this.dayBlock( day , open );
    if ( turn !== this.dayTurn ) return;
    holder.replaceChildren( block );
  },

  daySummary( day ) {
    const meta = [
      day.matches !== undefined && day.matches !== null ? I18n.format( "logs.matches" , { count: Number( day.matches ).toLocaleString() } )
        : I18n.format( "logs.entries" , { count: Number( day.entries || 0 ).toLocaleString() } ),
      this.size( day.bytes ),
    ];
    if ( day.compressed ) meta.push( I18n.get( "logs.compressed" ) );
    return Dom.el( "summary" , { class: "log-day-summary" , children: [
      Dom.el( "span" , { class: "log-day-date" , text: this.dayLabel( day.date ) } ),
      Dom.el( "span" , { class: "muted log-day-meta" , text: meta.join( " · " ) } ),
      day.errors > 0 ? Dom.el( "span" , { class: "log-badge log-badge-danger" , text: I18n.format( "logs.errors" , { count: day.errors } ) } ) : null,
    ].filter( Boolean ) } );
  },

  // A day, with its first page already in if it is open. Download and delete
  // sit inside the opened day, not on the summary line, where a click would
  // also fold the day.
  async dayBlock( day , open ) {
    const summary = this.daySummary( day );
    const body = Dom.el( "div" , { class: "log-day-body" } );
    const block = Dom.el( "details" , { class: "log-day" , attrs: { "data-date": day.date } , children: [ summary , body ] } );

    if ( day.missing ) {
      body.appendChild( Dom.el( "p" , { class: "muted log-day-none" , text: I18n.get( day.matches === undefined ? "logs.day_empty" : "logs.no_matches" ) } ) );
      block.open = open;
      return block;
    }

    const query = this.query();
    const download = Dom.el( "a" , {
      class: "log-button",
      text: I18n.get( "logs.download_button" ),
      attrs: {
        href: Api.logDownloadPath( day.date ) + ( query ? "?q=" + encodeURIComponent( query ) : "" ),
        download: "logs-" + day.date + ".jsonl",
      },
    } );
    const remove = day.date === this.today() ? null : Dom.el( "button" , {
      class: "small secondary log-delete",
      text: I18n.get( "logs.delete_button" ),
      attrs: { type: "button" },
      on: { click: () => this.onDeleteDay( day.date ) },
    } );
    body.appendChild( Dom.el( "div" , { class: "log-day-actions" , children: [ download , remove ].filter( Boolean ) } ) );

    if ( open ) {
      block._loaded = true;
      await this.loadDay( body , day.date , null );
    }
    block.addEventListener( "toggle" , () => {
      if ( block.open && !block._loaded ) {
        block._loaded = true;
        this.loadDay( body , day.date , null );
      }
    } );
    block.open = open;
    return block;
  },

  async loadDay( body , date , before ) {
    let page;
    try {
      page = await Api.logDay( date , this.query() , before );
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    let table = body.querySelector( "table" );
    if ( !table ) {
      const heads = [ "time" , "kind" , "user" , "what" , "ip" ].map( ( key ) =>
        Dom.el( "th" , { text: I18n.get( "logs.table_" + key ) , attrs: { scope: "col" } } ) );
      table = Dom.el( "table" , { class: "log-table" , children: [
        Dom.el( "thead" , { children: [ Dom.el( "tr" , { children: heads } ) ] } ),
        Dom.el( "tbody" ),
      ] } );
      body.appendChild( Dom.el( "div" , { class: "table-scroll" , children: [ table ] } ) );
    }
    const rows = table.querySelector( "tbody" );
    ( page.entries || [] ).forEach( ( entry ) => rows.append( ...this.entryRows( entry ) ) );

    const old = body.querySelector( ".log-more" );
    if ( old ) old.remove();
    if ( page.next !== null && page.next !== undefined && page.next >= 0 ) {
      body.appendChild( Dom.el( "button" , {
        class: "small secondary log-more",
        text: I18n.get( "logs.load_more" ),
        attrs: { type: "button" },
        on: { click: () => this.loadDay( body , date , page.next ) },
      } ) );
    }
  },

  tone( entry ) {
    if ( entry.level === "error" || entry.kind === "error" || ( entry.request && entry.request.status >= 500 ) ) return "danger";
    if ( entry.level === "warn" || ( entry.request && entry.request.status >= 400 ) ) return "warning";
    return { request: "plain" , ui: "info" , auth: "accent" , admin: "warning" , system: "dark" }[ entry.kind ] || "plain";
  },

  who( entry ) {
    const actor = entry.actor || {};
    if ( !actor.user_id ) return I18n.get( "logs.anonymous" );
    const name = actor.name || "#" + actor.user_id;
    return actor.method === "api_key" ? name + " " + I18n.get( "logs.via_key" ) : name;
  },

  what( entry ) {
    if ( entry.request ) {
      const request = entry.request;
      return request.method + " " + request.path + " → " + request.status + " · " + Math.round( request.ms * 10 ) / 10 + " ms";
    }
    if ( entry.ui ) {
      const ui = entry.ui;
      return [ ui.action , ui.target , ui.screen ? "· " + ui.screen : "" ].filter( Boolean ).join( " " );
    }
    return entry.msg || "";
  },

  // The address, or what it is called with the address under it. Pressing it
  // names it, rather than opening the row.
  ipCell( entry ) {
    if ( !entry.ip ) return Dom.el( "td" );
    const label = I18n.format( "logs.ip_name_button" , { ip: entry.ip } );
    return Dom.el( "td" , { children: [ Dom.el( "button" , {
      class: "log-ip",
      attrs: { type: "button" , "aria-label": label , title: label },
      children: entry.ip_name
        ? [ Dom.el( "span" , { text: entry.ip_name } ) , Dom.el( "span" , { class: "log-ip-address" , text: entry.ip } ) ]
        : [ Dom.el( "span" , { text: entry.ip } ) ],
      on: { click: ( event ) => {
        event.stopPropagation();
        this.startNaming( entry.ip , entry.ip_name || "" );
      } },
    } ) ] } );
  },

  // A row, and under it the whole entry as JSON, shown by clicking the row.
  entryRows( entry ) {
    const json = Dom.el( "pre" , { class: "log-json" , text: JSON.stringify( entry , null , 2 ) } );
    const copy = Dom.el( "button" , {
      class: "small secondary log-copy",
      text: I18n.get( "logs.copy" ),
      attrs: { type: "button" },
      on: { click: async () => {
        try {
          await navigator.clipboard.writeText( json.textContent );
          Toast.show( I18n.get( "logs.copied" ) , "ok" );
        } catch ( error ) {
          Toast.error( error );
        }
      } },
    } );
    const detail = Dom.el( "tr" , { class: "log-detail" , attrs: { hidden: "" } , children: [
      Dom.el( "td" , { attrs: { colspan: "5" } , children: [ copy , json ] } ),
    ] } );
    const time = new Date( entry.ts );
    const row = Dom.el( "tr" , {
      class: "log-row",
      attrs: { tabindex: "0" , "aria-expanded": "false" , "data-pos": String( entry.pos ) },
      children: [
        Dom.el( "td" , { class: "log-time" , text: Number.isNaN( time.getTime() ) ? "" : time.toLocaleTimeString() , attrs: { title: entry.ts } } ),
        Dom.el( "td" , { children: [ Dom.el( "span" , {
          class: "log-badge log-badge-" + this.tone( entry ),
          text: I18n.get( "logs.kind_" + entry.kind ) || entry.kind,
        } ) ] } ),
        Dom.el( "td" , { text: this.who( entry ) } ),
        Dom.el( "td" , { class: "log-what" , text: this.what( entry ) } ),
        this.ipCell( entry ),
      ],
    } );
    const toggle = () => {
      detail.hidden = !detail.hidden;
      row.setAttribute( "aria-expanded" , String( !detail.hidden ) );
    };
    row.addEventListener( "click" , toggle );
    row.addEventListener( "keydown" , ( event ) => {
      if ( event.target !== row ) return;
      if ( event.key === "Enter" || event.key === " " ) {
        event.preventDefault();
        toggle();
      }
    } );
    return [ row , detail ];
  },

  // ------------------------------------------------------ address names

  async refreshNames() {
    let listing;
    try {
      listing = await Api.ipNames();
    } catch ( error ) {
      if ( error.status !== 404 ) Toast.error( error );
      return;
    }
    const names = listing.names || [];
    Dom.text( Dom.get( "ip-names-summary" ) , I18n.format( "logs.ip_names_heading" , { count: names.length } ) );
    const table = Dom.get( "ip-names-table" );
    const body = table.querySelector( "tbody" );
    Dom.clear( body );
    names.forEach( ( item ) => {
      body.appendChild( Dom.el( "tr" , { children: [
        Dom.el( "td" , { children: [ Dom.el( "button" , {
          class: "log-ip",
          text: item.name,
          attrs: { type: "button" , title: I18n.format( "logs.ip_name_button" , { ip: item.target } ) },
          on: { click: () => this.startNaming( item.target , item.name ) },
        } ) ] } ),
        Dom.el( "td" , { class: "log-mono" , text: item.target } ),
        Dom.el( "td" , { class: "log-names-actions" , children: [ Dom.el( "button" , {
          class: "small secondary log-delete",
          text: I18n.get( "logs.ip_name_delete" ),
          attrs: { type: "button" },
          on: { click: () => this.onDeleteName( item ) },
        } ) ] } ),
      ] } ) );
    } );
    Dom.show( table , names.length > 0 );
    Dom.show( Dom.get( "ip-names-empty" ) , names.length === 0 );
  },

  // Opens the names card with the form filled in, ready for the name.
  startNaming( target , name ) {
    const card = Dom.get( "ip-names" );
    card.open = true;
    Dom.get( "ip-names-target" ).value = target;
    const field = Dom.get( "ip-names-name" );
    field.value = name;
    card.scrollIntoView( { block: "nearest" , behavior: Motion.reduced() ? "auto" : "smooth" } );
    field.focus( { preventScroll: true } );
    field.select();
  },

  async onSaveName() {
    const target = Dom.get( "ip-names-target" ).value.trim();
    const name = Dom.get( "ip-names-name" ).value.trim();
    if ( !target || !name ) return;
    try {
      await Api.setIpName( target , name );
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    Dom.get( "ip-names-target" ).value = "";
    Dom.get( "ip-names-name" ).value = "";
    await Promise.all( [ this.refresh() , this.refreshNames() ] );
    Toast.show( I18n.format( "logs.ip_named" , { name: name } ) , "ok" );
  },

  async onDeleteName( item ) {
    try {
      await Api.deleteIpName( item.target );
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    await Promise.all( [ this.refresh() , this.refreshNames() ] );
    Toast.show( I18n.get( "logs.ip_name_deleted" ) , "ok" );
  },

  // ------------------------------------------------------------ actions

  async onPrune() {
    const question = I18n.get( "logs.prune_confirm" );
    if ( question !== "" && !window.confirm( question ) ) return;
    try {
      const result = await Api.pruneLogs();
      Toast.show( I18n.format( "logs.pruned" , { deleted: result.deleted , compressed: result.compressed } ) , "ok" );
      await this.refresh();
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // No undo: the day's files are gone from disk.
  async onDeleteDay( date ) {
    const question = I18n.format( "logs.delete_confirm" , { date: this.dayLabel( date ) } );
    if ( question !== "" && !window.confirm( question ) ) return;
    try {
      await Api.deleteLogDay( date );
      this.followToday = true;
      await this.refresh();
      Toast.show( I18n.format( "logs.deleted" , { date: this.dayLabel( date ) } ) , "ok" );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // -------------------------------------------------------------- live

  setLive( on ) {
    this.liveOn = on;
    Dom.get( "logs-live" ).setAttribute( "aria-pressed" , String( on ) );
    this.followScreen();
  },

  // Each stream holds a connection, and a browser only gets a few per server.
  followScreen() {
    if ( this.liveOn && this.visible ) this.openLive();
    else this.closeLive();
  },

  openLive() {
    if ( this.liveSource ) return;
    let source;
    try {
      source = new EventSource( Api.logStreamPath() );
    } catch ( error ) {
      return;
    }
    source.addEventListener( "changed" , () => this.onChanged() );
    // A dropped connection retries by itself. CLOSED means the server said no:
    // signed out, or too many pages are watching.
    source.addEventListener( "error" , () => {
      if ( source.readyState !== EventSource.CLOSED ) return;
      this.setLive( false );
      Toast.show( I18n.get( "logs.live_lost" ) , "error" );
    } );
    this.liveSource = source;
  },

  closeLive() {
    if ( this.liveSource ) this.liveSource.close();
    this.liveSource = null;
    window.clearTimeout( this.liveTimer );
    this.liveTimer = null;
  },

  // At most one read per liveMs, and the last change always gets one. It holds
  // off while an entry is open, so what is being read does not fold away.
  onChanged() {
    if ( this.liveTimer ) return;
    const wait = Math.max( 0 , this.liveLast + this.liveMs - Date.now() );
    this.liveTimer = window.setTimeout( () => {
      this.liveTimer = null;
      this.liveLast = Date.now();
      if ( Dom.all( "#logs-day .log-detail:not([hidden])" ).length > 0 ) {
        this.onChanged();
        return;
      }
      this.liveRead();
    } , wait );
  },

  // Today's open day gets only the entries newer than its top row, added
  // above it; the rows already there stay. Anything else is a full refresh.
  async liveRead() {
    const block = Dom.get( "logs-day" ).querySelector( "details.log-day" );
    const rows = block && block.open && block._loaded && block.getAttribute( "data-date" ) === this.shown ? block.querySelector( ".log-table tbody" ) : null;
    const top = rows && rows.querySelector( "tr.log-row" );
    if ( !top || this.shown !== this.today() ) return this.refresh();
    const newest = Number( top.getAttribute( "data-pos" ) );
    let listing;
    let page;
    try {
      [ listing , page ] = await Promise.all( [ Api.logDays( this.query() ) , Api.logDay( this.shown , this.query() , null ) ] );
    } catch ( error ) {
      return;
    }
    const entries = page.entries || [];
    const fresh = entries.filter( ( entry ) => entry.pos > newest );
    // A whole page of new ones may have skipped some: start over.
    if ( fresh.length === entries.length && page.next !== null && page.next !== undefined && page.next >= 0 ) return this.refresh();
    this.renderSettings( listing );
    this.days = new Map( ( listing.days || [] ).map( ( day ) => [ day.date , day ] ) );
    this.renderCalendar();
    const day = this.days.get( this.shown );
    if ( day ) block.querySelector( "summary" ).replaceWith( this.daySummary( day ) );
    if ( fresh.length === 0 ) return;
    // Someone reading further down keeps the row they are on where it is.
    const before = document.scrollingElement.scrollHeight;
    const reading = rows.closest( "table" ).getBoundingClientRect().top < 0;
    rows.prepend( ...fresh.flatMap( ( entry ) => this.entryRows( entry ) ) );
    if ( reading ) window.scrollBy( 0 , document.scrollingElement.scrollHeight - before );
  },

  // ------------------------------------------------------------ helpers

  // Days are UTC dates (the files are named by them); times in the rows are
  // shown in the viewer's own zone.
  dayLabel( date ) {
    return new Date( date + "T00:00:00Z" ).toLocaleDateString( undefined , {
      weekday: "short", year: "numeric", month: "short", day: "numeric", timeZone: "UTC",
    } );
  },

  today() {
    return new Date().toISOString().slice( 0 , 10 );
  },

  size( bytes ) {
    const value = Number( bytes ) || 0;
    if ( value < 1024 ) return value + " B";
    if ( value < 1024 * 1024 ) return ( value / 1024 ).toFixed( 1 ) + " KB";
    return ( value / 1024 / 1024 ).toFixed( 1 ) + " MB";
  },
};
