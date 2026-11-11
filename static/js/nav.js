// The sidebar the lobby's hamburger opens. On a phone it slides in over the
// page, with a backdrop; Esc, the backdrop, the close button or following a
// link shuts it. On a wide screen it is a column beside the lobby instead,
// opened and closed by the same button, and that choice is remembered in
// this browser. Every link is a hash js/app.js already routes, so this file
// only opens, closes and marks the link for the screen showing.
const Nav = {
  me: null,
  // Wide enough that a column beside the lobby leaves room for it.
  wide: window.matchMedia( "(min-width: 64rem)" ),
  storageKey: "rook.sidebar",
  lastFocus: null,

  init( me ) {
    this.me = me;
    const sidebar = Dom.get( "sidebar" );
    sidebar.hidden = false;
    // Invites is for anyone allowed to invite (every admin included). The
    // Admin page under Settings -- players, game settings, everyone's API
    // keys -- and the activity log are the owner's alone, as on the server.
    Dom.show( Dom.get( "sidebar-invites" ) , me.can_invite === true );
    Dom.show( Dom.get( "sidebar-admin" ) , me.owner === true );
    this.renderUser();

    Dom.get( "sidebar-toggle" ).addEventListener( "click" , () => this.toggle() );
    Dom.get( "sidebar-close" ).addEventListener( "click" , () => this.close() );
    Dom.get( "sidebar-backdrop" ).addEventListener( "click" , () => this.close() );
    sidebar.addEventListener( "click" , ( event ) => {
      if ( event.target.closest( "a" ) && !this.wide.matches ) this.close();
    } );
    document.addEventListener( "keydown" , ( event ) => {
      if ( event.key === "Escape" && document.body.classList.contains( "sidebar-open" ) ) this.close();
    } );
    // Crossing the breakpoint with the panel open: the docked column takes
    // over from the slide-in, or the other way round.
    this.wide.addEventListener( "change" , () => {
      document.body.classList.remove( "sidebar-open" );
      this.apply();
    } );
    this.apply();
  },

  // Who is signed in, at the foot of the sidebar.
  renderUser() {
    if ( !this.me ) return;
    Dom.text( Dom.get( "sidebar-user" ) , this.me.display_name );
    Art.avatar( Dom.get( "sidebar-avatar" ) , this.me );
  },

  docked() {
    try {
      return window.localStorage.getItem( this.storageKey ) === "open";
    } catch ( error ) {
      return false;
    }
  },

  saveDocked( open ) {
    try {
      window.localStorage.setItem( this.storageKey , open ? "open" : "closed" );
    } catch ( error ) {
      // Private window or blocked storage: it works, it just is not remembered.
    }
  },

  toggle() {
    if ( this.wide.matches ) {
      this.saveDocked( !this.docked() );
      this.apply();
      return;
    }
    if ( document.body.classList.contains( "sidebar-open" ) ) {
      this.close();
    } else {
      this.open();
    }
  },

  open() {
    this.lastFocus = document.activeElement;
    document.body.classList.add( "sidebar-open" );
    Dom.get( "sidebar-backdrop" ).hidden = false;
    this.apply();
    Dom.get( "sidebar" ).focus();
  },

  close() {
    if ( this.wide.matches ) {
      this.saveDocked( false );
      this.apply();
      return;
    }
    document.body.classList.remove( "sidebar-open" );
    Dom.get( "sidebar-backdrop" ).hidden = true;
    this.apply();
    if ( this.lastFocus && document.contains( this.lastFocus ) ) this.lastFocus.focus();
    this.lastFocus = null;
  },

  // Body classes drive the CSS: sidebar-docked for the wide column,
  // sidebar-open for the phone's slide-in.
  apply() {
    const docked = this.wide.matches && this.docked();
    const open = !this.wide.matches && document.body.classList.contains( "sidebar-open" );
    document.body.classList.toggle( "sidebar-docked" , docked );
    if ( this.wide.matches ) Dom.get( "sidebar-backdrop" ).hidden = true;
    Dom.get( "sidebar-toggle" ).setAttribute( "aria-expanded" , String( docked || open ) );
  },

  // Called by the router on every screen change.
  mark( hash ) {
    Dom.all( "#sidebar [data-nav]" ).forEach( function ( link ) {
      const here = link.getAttribute( "data-nav" ) === hash;
      if ( here ) {
        link.setAttribute( "aria-current" , "page" );
      } else {
        link.removeAttribute( "aria-current" );
      }
    } );
    if ( !this.wide.matches && document.body.classList.contains( "sidebar-open" ) ) this.close();
  },
};
