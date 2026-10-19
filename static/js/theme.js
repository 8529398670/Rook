// Loaded in <head>, before any stylesheet, and run immediately: applying a
// saved dark choice here -- before first paint -- is what stops the page
// flashing light and then switching.
//
// Light is the default and stays the default. The app never follows the
// operating system's dark setting; dark only happens because someone picked
// it with the toggle in Settings, and that choice is remembered here.
//
// localStorage is a per-browser convenience, so every access is wrapped: a
// private window or blocked storage just means the default theme.
const Theme = {
  storageKey: "rook.theme",

  get() {
    try {
      return window.localStorage.getItem( this.storageKey ) === "dark" ? "dark" : "light";
    } catch ( error ) {
      return "light";
    }
  },

  apply( name ) {
    if ( name === "dark" ) {
      document.documentElement.setAttribute( "data-theme" , "dark" );
    } else {
      document.documentElement.removeAttribute( "data-theme" );
    }
  },

  set( name ) {
    try {
      window.localStorage.setItem( this.storageKey , name === "dark" ? "dark" : "light" );
    } catch ( error ) {
      // Storage refused: the choice still applies for this page view.
    }
    this.apply( name );
  },
};

Theme.apply( Theme.get() );

// Opened from the Home Screen on an iPad or iPhone (navigator.standalone is
// Apple's own flag for exactly that): iPadOS blurs a band under the clock
// that reaches further down than the safe area it reports, and blurs the top
// of the page with it. Marked here, before first paint, so style.css can
// start the page below that band without the page jumping.
if ( window.navigator.standalone === true ) document.documentElement.classList.add( "ios-app" );
