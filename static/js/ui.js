// Two small pieces of UI every screen shares: bottom sheets and toasts.
//
// A sheet's markup lives in index.html (so its text comes from language.yaml
// like everything else); this file only shows one, hides the rest, and
// handles closing -- the X in each sheet's corner, its own close buttons,
// and the Escape key. A tap outside a sheet does nothing: one stray tap
// should never throw away a half-filled form.
const Sheet = {
  current: null,
  onClose: null,

  init() {
    Dom.all( ".sheet" ).forEach( function ( sheet ) {
      sheet.insertBefore( Dom.el( "button" , {
        class: "icon-button sheet-close",
        attrs: { type: "button" , "data-sheet-close": "" , "aria-label": I18n.get( "common.close" ) , title: I18n.get( "common.close" ) },
        children: [ Dom.svg( "svg" , { viewBox: "0 0 24 24" , "aria-hidden": "true" } , [
          Dom.svg( "path" , { d: "M6 6l12 12M18 6L6 18" , fill: "none" , stroke: "currentColor" , "stroke-width": "2.4" , "stroke-linecap": "round" } ),
        ] ) ],
      } ) , sheet.firstChild );
    } );
    Dom.all( "[data-sheet-close]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () { Sheet.close(); } );
    } );
    document.addEventListener( "keydown" , function ( event ) {
      if ( event.key === "Escape" && Sheet.current && Sheet.dismissible ) Sheet.close();
    } );
  },

  // open( "sheet-rules", { onClose: fn, dismissible: false } ). A sheet that
  // waits on the player (the hand summary) is not dismissible: it has no X
  // and ignores Escape, so only its own buttons close it.
  open( id , options ) {
    const settings = options || {};
    if ( this.current && this.current !== id ) this.close( true );
    Dom.all( ".sheet" ).forEach( function ( sheet ) { sheet.hidden = sheet.id !== id; } );
    Dom.show( Dom.get( id ).querySelector( ".sheet-close" ) , settings.dismissible !== false );
    Dom.get( "sheet-layer" ).hidden = false;
    document.body.classList.add( "sheet-open" );
    this.current = id;
    this.onClose = settings.onClose || null;
    this.dismissible = settings.dismissible !== false;
    const focusable = Dom.get( id ).querySelector( "[autofocus], input, button:not(.sheet-close)" );
    if ( focusable && settings.focus !== false ) window.setTimeout( function () { focusable.focus( { preventScroll: true } ); } , 60 );
  },

  close( silently ) {
    if ( !this.current ) return;
    const callback = this.onClose;
    Dom.get( this.current ).hidden = true;
    Dom.get( "sheet-layer" ).hidden = true;
    document.body.classList.remove( "sheet-open" );
    this.current = null;
    this.onClose = null;
    if ( callback && !silently ) callback();
  },

  isOpen( id ) {
    return this.current === id;
  },
};

const Toast = {
  timer: null,

  show( message , kind ) {
    const toast = Dom.get( "toast" );
    if ( !message ) return;
    Dom.text( toast , message );
    toast.className = "toast" + ( kind ? " toast-" + kind : "" );
    toast.hidden = false;
    // Restart the entrance animation even if a toast is already showing.
    toast.classList.remove( "toast-in" );
    void toast.offsetWidth;
    toast.classList.add( "toast-in" );
    window.clearTimeout( this.timer );
    this.timer = window.setTimeout( function () { toast.hidden = true; } , kind === "error" ? 3800 : 2600 );
  },

  error( error ) {
    this.show( error && error.code ? I18n.error( error.code ) : I18n.get( "errors.generic" ) , "error" );
  },
};

// Motion is decoration here, never information, so anyone who has asked their
// system for less of it gets the end state straight away.
const Motion = {
  reduced() {
    return window.matchMedia && window.matchMedia( "(prefers-reduced-motion: reduce)" ).matches;
  },

  // Promise-returning Web Animations API call that resolves at once when
  // motion is reduced or the API is missing.
  animate( element , keyframes , options ) {
    if ( !element || this.reduced() || typeof element.animate !== "function" ) return Promise.resolve();
    const animation = element.animate( keyframes , options );
    return animation.finished.catch( function () {} );
  },

  wait( milliseconds ) {
    const duration = this.reduced() ? Math.min( milliseconds , 250 ) : milliseconds;
    return new Promise( function ( resolve ) { window.setTimeout( resolve , duration ); } );
  },
};
