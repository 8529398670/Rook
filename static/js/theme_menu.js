// The theme picker and Light/Dark switch in the table menu, so the theme can
// be changed in the middle of a game and seen on the table at once.
//
// Both act on the theme in use, the same way the Appearance screen does
// (themes_editor.js): picking one is Theme.pick, and Light/Dark is
// Theme.setMode on that theme, saved with the rest of the list, so each mode
// keeps its own colors and the Appearance screen shows the change too. Someone
// not signed in (watching a table) only changes this browser.
const ThemeMenu = {
  modes: [ "light" , "dark" ],
  buttons: [],

  init() {
    const select = Dom.get( "menu-theme" );
    const group = Dom.get( "menu-theme-mode" );
    if ( !select || !group ) return;
    select.addEventListener( "change" , () => {
      Theme.pick( select.value ).catch( ( error ) => { Toast.error( error ); this.render(); } );
    } );
    this.buttons = this.modes.map( ( mode ) => {
      const button = Dom.el( "button" , {
        text: I18n.get( "theme.mode_" + mode ),
        attrs: { type: "button" , role: "radio" , "data-mode": mode },
        on: { click: () => this.setMode( mode ) },
      } );
      group.appendChild( button );
      return button;
    } );
    Theme.onChange.push( () => this.render() );
    this.render();
  },

  render() {
    const select = Dom.get( "menu-theme" );
    if ( !select ) return;
    select.replaceChildren( ...Theme.list.map( ( theme ) => Dom.el( "option" , {
      text: theme.name || I18n.get( "theme.preset_" + theme.id ) || theme.id,
      attrs: { value: theme.id },
    } ) ) );
    select.value = Theme.current;
    const mode = Theme.find( Theme.current ).mode;
    this.buttons.forEach( ( button ) => {
      button.setAttribute( "aria-checked" , String( button.getAttribute( "data-mode" ) === mode ) );
    } );
  },

  async setMode( mode ) {
    const current = Theme.find( Theme.current );
    if ( current.mode === mode ) return;
    if ( !Theme.signedIn ) {
      Theme.setMode( current , mode );
      Theme.show();
      return;
    }
    const next = Theme.list.map( ( theme ) => {
      const copy = Theme.copy( theme );
      if ( theme.id === current.id ) Theme.setMode( copy , mode );
      return copy;
    } );
    // Paint it now; the save's reply repaints from the account.
    Theme.apply( next.find( ( theme ) => theme.id === current.id ) );
    try {
      await Theme.store( next , current.id );
    } catch ( error ) {
      Toast.error( error );
      Theme.show();
    }
  },
};
