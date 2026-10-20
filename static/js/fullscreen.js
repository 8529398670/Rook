// Full screen: hands the whole display to the page, so a small laptop gets
// back the rows the browser's tabs and toolbars take. Every control marked
// data-fullscreen toggles it; data-fullscreen-label holds the text to swap
// on a labelled one (the table menu's row). The F key toggles it too.
//
// Where the browser cannot do it (an iPhone, or a page in a frame that does
// not allow it) the controls are hidden rather than left to do nothing.
const Fullscreen = {
  supported() {
    const root = document.documentElement;
    return Boolean( document.fullscreenEnabled || document.webkitFullscreenEnabled ) &&
      Boolean( root.requestFullscreen || root.webkitRequestFullscreen );
  },

  active() {
    return Boolean( document.fullscreenElement || document.webkitFullscreenElement );
  },

  init() {
    const supported = this.supported();
    Dom.all( "[data-fullscreen]" ).forEach( function ( control ) {
      if ( !supported ) {
        control.hidden = true;
        control.classList.add( "is-unsupported" );
        return;
      }
      control.addEventListener( "click" , function () {
        Fullscreen.toggle();
        // From the table menu: the menu has done its job.
        if ( control.closest( ".sheet" ) ) Sheet.close();
      } );
    } );
    if ( !supported ) return;
    // F toggles it from anywhere, except while typing in a text field.
    document.addEventListener( "keydown" , function ( event ) {
      if ( event.ctrlKey || event.metaKey || event.altKey || event.repeat ) return;
      if ( event.key !== "f" && event.key !== "F" ) return;
      const target = event.target;
      if ( target && ( target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test( target.tagName ) ) ) return;
      event.preventDefault();
      Fullscreen.toggle();
    } );
    document.addEventListener( "fullscreenchange" , this.sync.bind( this ) );
    document.addEventListener( "webkitfullscreenchange" , this.sync.bind( this ) );
    this.sync();
  },

  async toggle() {
    const root = document.documentElement;
    try {
      if ( this.active() ) {
        if ( document.exitFullscreen ) await document.exitFullscreen();
        else document.webkitExitFullscreen();
      } else if ( root.requestFullscreen ) {
        await root.requestFullscreen( { navigationUI: "hide" } );
      } else {
        root.webkitRequestFullscreen();
      }
    } catch ( error ) {
      // Refused (no user gesture, or a browser policy): stay as we are.
    }
  },

  sync() {
    const on = this.active();
    const label = I18n.get( on ? "table.exit_fullscreen" : "table.fullscreen" );
    Dom.all( "[data-fullscreen]" ).forEach( function ( control ) {
      control.setAttribute( "aria-pressed" , String( on ) );
      const text = control.querySelector( "[data-fullscreen-label]" );
      if ( text ) {
        Dom.text( text , label );
      } else {
        control.setAttribute( "aria-label" , label );
        control.setAttribute( "title" , label );
      }
    } );
  },
};
