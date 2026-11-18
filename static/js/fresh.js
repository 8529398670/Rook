// A newer build, heard while this tab is open.
//
// Every API reply names the build that answered (X-App-Version, read by
// Api.request). Once a newer build is known, the next move to another screen
// is a real page load instead of an in-page switch, landing on the screen
// asked for, now running the new code -- nobody has to press refresh. If they
// stay put, a toast says so after toastAfterMs. Nothing reloads under an open
// sheet, or while a screen registered with holdWhile says it would lose
// something.
//
// Unlike the template's copy, this one does not ask /api/me again when a tab
// comes back: app.js is the one place that checks the session.
const Fresh = {
  // How long a newer build waits for a screen change before saying so.
  toastAfterMs: 60 * 1000,
  version: "",
  newer: "",
  offered: "",
  holds: [],

  init( me ) {
    this.version = me.version;
  },

  // A screen with something a reload would lose says so here; test answers
  // true while it does.
  holdWhile( test ) {
    this.holds.push( test );
  },

  // Api.request passes the build that answered each call. dev is what a build
  // with no version stamped says, which is every build of one being worked
  // on, so a change to or from it means nothing (static/js/live.js covers
  // that case).
  heard( version ) {
    if ( !version || !this.version || version === "dev" || this.version === "dev" ) return;
    if ( version !== this.version ) this.notice( version );
  },

  // Something newer than this page is known: a build (its version), or a
  // file changed under it (static/js/live.js says "live"). The next screen
  // change reloads; if none comes, the toast says so.
  notice( what ) {
    if ( what === this.newer ) return;
    this.newer = what;
    window.setTimeout( () => this.offer() , this.toastAfterMs );
  },

  safe() {
    return !Sheet.current && !this.holds.some( ( test ) => test() );
  },

  // app.js asks before showing another screen. True means the page is
  // reloading onto the screen in the URL, so there is nothing to show.
  swap() {
    if ( !this.newer || !this.safe() ) return false;
    window.location.reload();
    return true;
  },

  // Told once: someone who ignores the toast has seen it.
  offer() {
    if ( this.offered === this.newer ) return;
    this.offered = this.newer;
    Toast.show( I18n.get( "fresh.new_version" ) );
  },
};
