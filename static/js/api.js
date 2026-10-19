// Every call to the server goes through here, so the things that are easy to
// forget -- sending the cookie, attaching the CSRF token, turning a non-2xx
// response into a real Error -- happen once instead of at each call site.
const Api = {
  csrfToken: null,
  statusCodes: { 401: "signed_out" , 403: "forbidden" , 429: "rate_limited" },

  async request( path , options ) {
    const settings = options || {};
    const response = await fetch( path , {
      method: settings.method || "GET",
      // The session cookie is the only credential; without this an
      // authenticated request silently arrives as an anonymous one.
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: settings.body ? JSON.stringify( settings.body ) : undefined,
    } );

    let payload = {};
    try {
      payload = await response.json();
    } catch ( parseError ) {
      payload = {};
    }

    if ( !response.ok ) {
      const error = new Error( payload.error || response.statusText );
      error.status = response.status;
      // Game and table refusals are codes ("must_follow_suit"), which the
      // page turns into a sentence with I18n.error( code ). The template's
      // own refusals are short English phrases instead; for those the HTTP
      // status is the useful part.
      const code = String( payload.error || "" );
      error.code = /^[a-z_]+$/.test( code ) ? code : ( this.statusCodes[ response.status ] || "generic" );
      throw error;
    }
    return payload;
  },

  // Any state-changing call carries the per-session CSRF token in the body.
  // Wrapping it here means a new endpoint cannot forget it.
  async post( path , body ) {
    return this.request( path , {
      method: "POST",
      body: Object.assign( {} , body || {} , { csrf_token: this.csrfToken } ),
    } );
  },

  async me() {
    try {
      const payload = await this.request( "/api/me" );
      this.csrfToken = payload.csrf_token;
      return payload;
    } catch ( error ) {
      if ( error.status === 401 ) return { authenticated: false };
      throw error;
    }
  },

  rename( displayName )        { return this.post( "/api/account/rename" , { display_name: displayName } ); },
  logout()                     { return this.post( "/api/logout" ); },
  listTeam()                   { return this.request( "/api/team" ); },
  listUsers()                  { return this.request( "/api/admin/users" ); },
  createUser( name , role )    { return this.post( "/api/admin/users" , { display_name: name , role: role } ); },
  reissueLogin( userId )       { return this.post( "/api/admin/users/" + userId + "/reissue-login" ); },
  setDisabled( userId , flag ) { return this.post( "/api/admin/users/" + userId + "/disabled" , { disabled: flag } ); },
  adminRename( userId , name ) { return this.post( "/api/admin/users/" + userId + "/rename" , { display_name: name } ); },
  adminSetAvatar( userId , icon , color ) {
    return this.post( "/api/admin/users/" + userId + "/avatar" , { icon: icon , color: color } );
  },
  deleteUser( userId )         { return this.post( "/api/admin/users/" + userId + "/delete" ); },

  // The game history: finished and abandoned games, newest first.
  listGames( offset )          { return this.request( "/api/games?offset=" + ( offset || 0 ) ); },
  getGame( id )                { return this.request( "/api/games/" + encodeURIComponent( id ) ); },

  // API keys. createKey's expiresInDays is passed through as-is including null,
  // because the server reads absent as "use the default" and 0 as "never
  // expires" -- collapsing them here would lose the distinction.
  listKeys()                   { return this.request( "/api/keys" ); },
  createKey( name , role , expiresInDays ) {
    return this.post( "/api/keys" , { name: name , role: role , expires_in_days: expiresInDays } );
  },
  revokeKey( keyId )           { return this.post( "/api/keys/" + encodeURIComponent( keyId ) + "/revoke" ); },
  listAllKeys()                { return this.request( "/api/admin/keys" ); },
  revokeAnyKey( keyId )        { return this.post( "/api/admin/keys/" + encodeURIComponent( keyId ) + "/revoke" ); },
  setCanInvite( userId , flag ) { return this.post( "/api/admin/users/" + userId + "/can-invite" , { can_invite: flag } ); },

  setCanHint( userId , flag )  { return this.post( "/api/admin/users/" + userId + "/can-hint" , { can_hint: flag } ); },
  getSettings()                { return this.request( "/api/admin/settings" ); },
  saveSettings( settings )     { return this.post( "/api/admin/settings" , settings ); },
  // Sign-up links: anyone who may invite manages their own; admins see all.
  listSignupLinks()            { return this.request( "/api/signup-links" ); },
  createSignupLink( label , maxUses , days ) {
    return this.post( "/api/signup-links" , { label: label , max_uses: maxUses , days: days } );
  },
  revokeSignupLink( id )       { return this.post( "/api/signup-links/" + encodeURIComponent( id ) + "/revoke" ); },

  // A one-time link that signs another device in as you.
  myLoginLink()                { return this.post( "/api/account/login-link" ); },

  // Your own look and PIN.
  profileOptions()             { return this.request( "/api/profile/options" ); },
  setAvatar( icon , color )    { return this.post( "/api/account/avatar" , { icon: icon , color: color } ); },
  setPin( pin )                { return this.post( "/api/account/pin" , { pin: pin } ); },

  // Join links and multi-use sign-up links. The sign-up credential goes in
  // the body, never the URL, so it stays out of server logs.
  joinInfo( code )             { return this.request( "/api/join/" + encodeURIComponent( code ) ); },
  joinTable( code )            { return this.post( "/api/join/" + encodeURIComponent( code ) ); },
  joinSignup( code , name )    { return this.post( "/api/join/" + encodeURIComponent( code ) + "/signup" , { display_name: name } ); },
  signupInfo( credential )     { return this.post( "/api/signup/info" , { credential: credential } ); },
  signupMatch( credential , name ) { return this.post( "/api/signup/match" , { credential: credential , display_name: name } ); },
  signupClaim( credential , userId , pin ) { return this.post( "/api/signup/claim" , { credential: credential , user_id: userId , pin: pin || "" } ); },
  signupCreate( credential , name ) { return this.post( "/api/signup/create" , { credential: credential , display_name: name } ); },

  // Card tables. Every action answers with the caller's fresh table view, so
  // the screen can update at once rather than waiting for the stream. The
  // query is a watcher's ?seat=N&all=1, kept on every call so the answer is
  // drawn from the seat they are watching.
  tableQuery: "",
  tablePath( id , action ) {
    return "/api/tables/" + encodeURIComponent( id ) + ( action ? "/" + action : "" ) + this.tableQuery;
  },
  watchPath( code , events ) {
    return "/api/watch/" + encodeURIComponent( code ) + ( events ? "/events" : "" ) + this.tableQuery;
  },
  listTables()                 { return this.request( "/api/tables" ); },
  createTable( name , options ) { return this.post( "/api/tables" , { name: name , options: options } ); },
  getTable( id )               { return this.request( this.tablePath( id ) ); },
  getWatch( code )             { return this.request( this.watchPath( code ) ); },
  hint( id )                   { return this.request( this.tablePath( id , "hint" ) ); },
  history( id )                { return this.request( this.tablePath( id , "history" ) ); },
  undo( id )                   { return this.post( this.tablePath( id , "undo" ) ); },
  redo( id )                   { return this.post( this.tablePath( id , "redo" ) ); },
  resetTo( id , to )           { return this.post( this.tablePath( id , "reset" ) , { to: to } ); },
  resume( id )                 { return this.post( this.tablePath( id , "resume" ) ); },
  setObservers( id , seeAll )  { return this.post( this.tablePath( id , "observers" ) , { see_all: seeAll } ); },
  sit( id , seat )             { return this.post( this.tablePath( id , "sit" ) , { seat: seat } ); },
  leave( id )                  { return this.post( this.tablePath( id , "leave" ) ); },
  addBot( id , seat )          { return this.post( this.tablePath( id , "bot" ) , { seat: seat } ); },
  removeBot( id , seat )       { return this.post( this.tablePath( id , "bot/remove" ) , { seat: seat } ); },
  invite( id , seat , name )   { return this.post( this.tablePath( id , "invite" ) , { seat: seat , display_name: name } ); },
  assignSeat( id , seat , userId ) { return this.post( this.tablePath( id , "assign" ) , { seat: seat , user_id: userId } ); },
  removePlayer( id , seat )    { return this.post( this.tablePath( id , "seat/remove" ) , { seat: seat } ); },
  renameSeat( id , seat , name ) { return this.post( this.tablePath( id , "seat/rename" ) , { seat: seat , name: name } ); },
  // dealer: a seat number or "random" for a new table's first deal; left
  // out, whoever pressed Deal deals (and a rematch just moves the deal on).
  start( id , dealer ) {
    const body = dealer === "random" ? { random_dealer: true } : ( typeof dealer === "number" ? { dealer: dealer } : {} );
    return this.post( this.tablePath( id , "start" ) , body );
  },
  restart( id )                { return this.post( this.tablePath( id , "restart" ) ); },
  deleteTable( id )            { return this.post( this.tablePath( id , "delete" ) ); },
  bid( id , amount )           { return this.post( this.tablePath( id , "bid" ) , { amount: amount } ); },
  pass( id )                   { return this.post( this.tablePath( id , "bid" ) , { amount: 0 } ); },
  bury( id , cards , trump )   { return this.post( this.tablePath( id , "bury" ) , { cards: cards , trump: trump } ); },
  play( id , card )            { return this.post( this.tablePath( id , "play" ) , { card: card } ); },
  continueHand( id )           { return this.post( this.tablePath( id , "continue" ) ); },
  chat( id , text )            { return this.post( this.tablePath( id , "chat" ) , { text: text } ); },
  watchChat( code , name , text ) {
    return this.post( "/api/watch/" + encodeURIComponent( code ) + "/chat" + this.tableQuery , { name: name , text: text } );
  },

  // Taking over a bot's seat: a watcher asks, the host (or an admin) answers.
  requestSeat( id , seat )     { return this.post( this.tablePath( id , "takeover" ) , { seat: seat } ); },
  cancelRequest( id )          { return this.post( this.tablePath( id , "takeover/cancel" ) ); },
  answerRequest( id , userId , accept ) {
    return this.post( this.tablePath( id , "takeover/answer" ) , { user_id: userId , accept: accept } );
  },

  // The whole game so far, hand by hand, for the game log.
  record( id )                 { return this.request( "/api/tables/" + encodeURIComponent( id ) + "/record" ); },
  watchRecord( code )          { return this.request( "/api/watch/" + encodeURIComponent( code ) + "/record" ); },
};
