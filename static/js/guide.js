// The Rules, Strategy, Bots and About pages, opened from the lobby's ⓘ button:
// #/rules, #/strategy, #/bots and #/about. All are drawn from language.yaml
// (guide.rules, guide.strategy, guide.bots, guide.about), a list of sections
// each with a heading, an optional lead, a glossary, bullet points, a row of
// card faces, a "rule of thumb", plain paragraphs and links to read more -- so
// the whole guide is reworded, reordered or trimmed there, never here.
//
// The tabs swap pages with replaceState rather than a new history entry, so
// Back always returns to wherever the guide was opened from. Nothing here
// talks to the server, which is why the guide is open to signed-out visitors
// too.
const Guide = {
  screenId: "screen-guide",
  tabs: [ "rules" , "strategy" , "bots" , "about" ],
  bound: false,
  tab: "rules",

  bind() {
    if ( this.bound ) return;
    this.bound = true;
    Dom.get( "guide-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.all( "[data-guide-tab]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        const tab = button.getAttribute( "data-guide-tab" );
        if ( tab === Guide.tab ) return;
        window.history.replaceState( null , "" , "#/" + tab );
        Guide.show( tab );
      } );
    } );
  },

  enter( tab ) {
    this.bind();
    this.show( tab );
  },

  leave() {},

  show( tab ) {
    this.tab = this.tabs.indexOf( tab ) >= 0 ? tab : "rules";
    Dom.all( "[data-guide-tab]" ).forEach( function ( button ) {
      button.setAttribute( "aria-selected" , String( button.getAttribute( "data-guide-tab" ) === Guide.tab ) );
    } );
    document.title = I18n.get( "guide." + this.tab + "_tab" ) + " · " + I18n.get( "app.title" );
    this.render();
    window.scrollTo( 0 , 0 );
  },

  render() {
    const body = Dom.get( "guide-body" );
    const jumps = Dom.get( "guide-jumps" );
    Dom.clear( body );
    Dom.clear( jumps );
    const tab = this.tab;
    I18n.list( "guide." + tab ).forEach( function ( section , index ) {
      if ( !section || typeof section.heading !== "string" || section.heading === "" ) return;
      const id = "guide-" + tab + "-" + ( section.id || index );
      const node = Guide.section( section , id );
      body.appendChild( node );
      jumps.appendChild( Dom.el( "li" , { children: [
        Dom.el( "button" , { class: "chip guide-jump" , text: section.heading , attrs: { type: "button" } , on: { click: function () { Guide.jump( node ); } } } ),
      ] } ) );
    } );
    Dom.show( Dom.get( "guide-contents" ) , jumps.childElementCount > 1 );
  },

  section( section , id ) {
    const strings = function ( value ) {
      return Array.isArray( value ) ? value.filter( function ( item ) { return typeof item === "string" && item !== ""; } ) : [];
    };
    const children = [ Dom.el( "h2" , { text: section.heading } ) ];
    if ( typeof section.lead === "string" && section.lead !== "" ) {
      children.push( Dom.el( "p" , { class: "guide-lead" , text: section.lead } ) );
    }
    strings( section.paragraphs ).forEach( function ( text ) {
      children.push( Dom.el( "p" , { class: "guide-paragraph" , text: text } ) );
    } );

    const cards = strings( section.cards );
    const labels = Array.isArray( section.labels ) ? section.labels : [];
    if ( cards.length > 0 ) {
      children.push( Dom.el( "div" , { class: "guide-cards" , children: cards.map( function ( card , at ) {
        const parts = [ Cards.el( card , { class: "guide-card" } ) ];
        if ( typeof labels[ at ] === "string" && labels[ at ] !== "" ) {
          parts.push( Dom.el( "figcaption" , { text: labels[ at ] } ) );
        }
        return Dom.el( "figure" , { children: parts } );
      } ) } ) );
    }

    const terms = Array.isArray( section.terms ) ? section.terms.filter( function ( entry ) {
      return entry && typeof entry.term === "string" && entry.term !== "" && typeof entry.meaning === "string" && entry.meaning !== "";
    } ) : [];
    if ( terms.length > 0 ) {
      const list = Dom.el( "dl" , { class: "guide-terms" } );
      terms.forEach( function ( entry ) {
        list.appendChild( Dom.el( "div" , { children: [
          Dom.el( "dt" , { text: entry.term } ),
          Dom.el( "dd" , { text: entry.meaning } ),
        ] } ) );
      } );
      children.push( list );
    }

    const points = strings( section.points );
    if ( points.length > 0 ) {
      children.push( Dom.el( "ul" , { class: "guide-points" , children: points.map( function ( point ) {
        return Dom.el( "li" , { text: point } );
      } ) } ) );
    }

    if ( typeof section.tip === "string" && section.tip !== "" ) {
      const label = I18n.get( "guide.rule_of_thumb" );
      children.push( Dom.el( "p" , { class: "guide-tip" , children: [
        label !== "" ? Dom.el( "strong" , { text: label } ) : null,
        Dom.el( "span" , { text: section.tip } ),
      ].filter( Boolean ) } ) );
    }

    // Links open in a new tab, and only to web addresses: a typo in the
    // YAML cannot turn one into a script.
    const links = Array.isArray( section.links ) ? section.links.filter( function ( entry ) {
      return entry && typeof entry.text === "string" && entry.text !== "" && /^https?:\/\//.test( entry.url || "" );
    } ) : [];
    if ( links.length > 0 ) {
      children.push( Dom.el( "ul" , { class: "guide-links" , children: links.map( function ( entry ) {
        return Dom.el( "li" , { children: [
          Dom.el( "a" , { class: "guide-link" , text: entry.text , attrs: { href: entry.url , target: "_blank" , rel: "noopener" } } ),
        ] } );
      } ) } ) );
    }

    const highlight = section.style === "highlight" ? " guide-highlight" : "";
    return Dom.el( "section" , { class: "card guide-section" + highlight , attrs: { id: id } , children: children } );
  },

  jump( node ) {
    const still = window.matchMedia && window.matchMedia( "(prefers-reduced-motion: reduce)" ).matches;
    node.scrollIntoView( { behavior: still ? "auto" : "smooth" , block: "start" } );
  },
};
