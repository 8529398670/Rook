// A multi-use sign-up link (#/signup/<credential>). You type your name every
// time. If it is close to a name that already joined through this link you
// are asked "did you mean...?" -- pick yourself and you are signed in on this
// device (after your PIN, if you set one). Otherwise you get a new account,
// while the link has places left.
const Signup = {
  screenId: "screen-signup",
  credential: null,
  name: "",
  picked: null,

  init() {
    Dom.get( "signup-name-form" ).addEventListener( "submit" , this.onName.bind( this ) );
    Dom.get( "signup-pin-form" ).addEventListener( "submit" , this.onPin.bind( this ) );
    Dom.get( "signup-new" ).addEventListener( "click" , this.create.bind( this ) );
    Dom.get( "signup-back" ).addEventListener( "click" , this.step.bind( this , "name" ) );
    Dom.get( "signup-pin-back" ).addEventListener( "click" , this.step.bind( this , "matches" ) );
  },

  step( which ) {
    Dom.get( "signup-name-form" ).hidden = which !== "name";
    Dom.get( "signup-matches" ).hidden = which !== "matches";
    Dom.get( "signup-pin-form" ).hidden = which !== "pin";
    Dom.get( "signup-error" ).hidden = true;
    const focus = { name: "signup-name" , pin: "signup-pin" }[ which ];
    if ( focus ) Dom.get( focus ).focus();
  },

  showError( code ) {
    const box = Dom.get( "signup-error" );
    Dom.text( box , I18n.error( code ) );
    box.hidden = false;
  },

  async enter( credential ) {
    this.credential = credential;
    this.picked = null;
    this.step( "name" );
    Dom.text( Dom.get( "signup-label" ) , "" );
    const signedIn = Dom.get( "signup-signed-in" );
    signedIn.hidden = !App.me;
    if ( App.me ) Dom.text( signedIn , I18n.format( "signup.signed_in_as" , { name: App.me.display_name } ) );
    try {
      const info = await Api.signupInfo( credential );
      Dom.text( Dom.get( "signup-label" ) , info.label );
    } catch ( error ) {
      Dom.get( "signup-name-form" ).hidden = true;
      this.showError( error.code );
    }
  },

  leave() {},

  async onName( event ) {
    event.preventDefault();
    this.name = Dom.get( "signup-name" ).value.trim();
    if ( !this.name ) return;
    let result = null;
    try {
      result = await Api.signupMatch( this.credential , this.name );
    } catch ( error ) {
      this.showError( error.code );
      return;
    }
    if ( result.matches.length === 0 && !result.can_create ) {
      this.showError( "signup_link_full" );
      return;
    }

    // With no close names, still ask before creating: every new account
    // uses up one of the link's places, and a typo should not.
    Dom.text( Dom.get( "signup-question" ) , result.matches.length
      ? I18n.get( "signup.did_you_mean" )
      : I18n.format( "signup.no_match" , { name: this.name } ) );
    const list = Dom.get( "signup-match-list" );
    Dom.clear( list );
    result.matches.forEach( function ( match ) {
      const avatar = Dom.el( "span" , { class: "avatar" , attrs: { "aria-hidden": "true" } } );
      Art.avatar( avatar , match );
      list.appendChild( Dom.el( "button" , {
        class: "match-button secondary",
        attrs: { type: "button" },
        children: [
          avatar,
          Dom.el( "span" , { class: "match-name" , text: match.display_name } ),
          Dom.el( "span" , { class: "match-note" , text: match.has_pin ? I18n.get( "signup.has_pin" ) : I18n.get( "signup.this_is_me" ) } ),
        ],
        on: { click: function () { Signup.pick( match ); } },
      } ) );
    } );
    const fresh = Dom.get( "signup-new" );
    fresh.hidden = !result.can_create;
    Dom.text( fresh , I18n.format( result.matches.length ? "signup.create_new" : "signup.create_first" , { name: this.name } ) );
    this.step( "matches" );
  },

  pick( match ) {
    this.picked = match;
    if ( match.has_pin ) {
      Dom.text( Dom.get( "signup-pin-for" ) , I18n.format( "signup.pin_for" , { name: match.display_name } ) );
      Dom.get( "signup-pin" ).value = "";
      this.step( "pin" );
      return;
    }
    this.claim( "" );
  },

  onPin( event ) {
    event.preventDefault();
    this.claim( Dom.get( "signup-pin" ).value.trim() );
  },

  async claim( pin ) {
    try {
      await Api.signupClaim( this.credential , this.picked.id , pin );
      this.done();
    } catch ( error ) {
      this.showError( error.code );
    }
  },

  async create() {
    try {
      await Api.signupCreate( this.credential , this.name );
      this.done();
    } catch ( error ) {
      this.showError( error.code );
    }
  },

  // Signed in now (the server set the cookie): start the app over as them.
  done() {
    window.location.hash = "#/";
    window.location.reload();
  },
};
