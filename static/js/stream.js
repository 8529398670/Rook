// Live updates from the server, over Server-Sent Events.
//
// Every message is a complete snapshot, so this file has nothing to merge or
// replay: it hands each snapshot to the page, and the page draws it. The
// server ends each stream after ~20 seconds and EventSource reconnects on its
// own; this wrapper adds two things on top:
//
//   - When a phone comes back from the background, reconnect at once rather
//     than waiting out the browser's backoff, and fetch a fresh snapshot.
//   - Tell the page when the connection is down, so it can say so instead of
//     silently showing a table that has stopped moving.
const Stream = {
  // open( url, { table: fn, lobby: fn, gone: fn, status: fn( connected ) } )
  open( url , handlers ) {
    let source = null;
    let closed = false;
    let downTimer = null;
    let retryTimer = null;

    const status = function ( connected ) {
      window.clearTimeout( downTimer );
      if ( connected ) {
        if ( handlers.status ) handlers.status( true );
        return;
      }
      // A planned reconnect every 20s is not an outage; only report one
      // that lasts.
      downTimer = window.setTimeout( function () {
        if ( !closed && handlers.status ) handlers.status( false );
      } , 4000 );
    };

    const connect = function () {
      if ( closed ) return;
      if ( source ) source.close();
      source = new EventSource( url , { withCredentials: true } );
      [ "table" , "lobby" ].forEach( function ( name ) {
        source.addEventListener( name , function ( event ) {
          status( true );
          let payload = null;
          try {
            payload = JSON.parse( event.data );
          } catch ( error ) {
            return;
          }
          if ( handlers[ name ] ) handlers[ name ]( payload );
        } );
      } );
      source.addEventListener( "gone" , function () {
        stop();
        if ( handlers.gone ) handlers.gone();
      } );
      source.addEventListener( "error" , function ( event ) {
        status( false );
        // A refusal (a 429, a 5xx) closes an EventSource for good -- it
        // only retries dropped connections. Try again ourselves, or the
        // table would sit frozen until the phone next comes to the front.
        if ( event.target.readyState === EventSource.CLOSED ) {
          window.clearTimeout( retryTimer );
          retryTimer = window.setTimeout( connect , 3000 );
        }
      } );
    };

    const onVisible = function () {
      if ( document.visibilityState === "visible" && !closed ) connect();
    };

    const stop = function () {
      closed = true;
      window.clearTimeout( downTimer );
      window.clearTimeout( retryTimer );
      document.removeEventListener( "visibilitychange" , onVisible );
      if ( source ) source.close();
    };

    document.addEventListener( "visibilitychange" , onVisible );
    connect();
    return { close: stop };
  },
};
