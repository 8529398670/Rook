package game

// finishHand scores a completed hand.
//
// Three things the original version got wrong, and the tests pin down:
//   - The kitty belongs to whichever team takes the *last trick* -- not to a
//     fixed team.
//   - A bidding team that falls short loses its bid, and the defenders still
//     score their own counters -- not the bidders'.
//   - The game ends when *either* team reaches the target. If both get there
//     on the same hand the higher total wins, and a tie goes to the team that
//     took the bid, since they carried the risk.
func ( state *State ) finishHand() {
	last := state.Completed[ len( state.Completed )-1 ]
	kitty_team := TeamOf( last.Winner )
	kitty_points := 0
	for _ , card := range state.Kitty {
		kitty_points += card.Points()
	}
	state.Taken[ kitty_team ] += kitty_points

	bidding_team := TeamOf( state.Bidder )
	defending_team := 1 - bidding_team
	made := state.Taken[ bidding_team ] >= state.Bid

	var scored [2]int
	scored[ defending_team ] = state.Taken[ defending_team ]
	if made {
		scored[ bidding_team ] = state.Taken[ bidding_team ]
	} else {
		scored[ bidding_team ] = -state.Bid
	}
	state.Scores[ 0 ] += scored[ 0 ]
	state.Scores[ 1 ] += scored[ 1 ]

	state.History = append( state.History , HandResult{
		Hand:        state.HandNumber,
		Bidder:      state.Bidder,
		Bid:         state.Bid,
		Trump:       state.Trump,
		Taken:       state.Taken,
		Scored:      scored,
		Totals:      state.Scores,
		TricksWon:   state.TricksWon,
		Made:        made,
		Kitty:       append( []Card( nil ) , state.Kitty... ),
		KittyPoints: kitty_points,
		KittyTeam:   kitty_team,
	} )

	made_amount := 0
	if made { made_amount = 1 }
	state.log( Event{ Kind: "hand_over" , Seat: state.Bidder , Amount: made_amount , Team: bidding_team } )

	state.Trick = Trick{ Leader: -1 , Winner: -1 }
	state.Turn = -1
	state.Ready = [4]bool{}

	reached := state.Options.PlayTo == 0 ||
		state.Scores[ 0 ] >= state.Options.PlayTo ||
		state.Scores[ 1 ] >= state.Options.PlayTo
	if reached == false {
		state.Phase = PhaseHandOver
		return
	}

	switch {
	case state.Scores[ 0 ] > state.Scores[ 1 ]:
		state.Winner = 0
	case state.Scores[ 1 ] > state.Scores[ 0 ]:
		state.Winner = 1
	default:
		state.Winner = bidding_team
	}
	state.Phase = PhaseGameOver
	state.log( Event{ Kind: "game_over" , Seat: -1 , Team: state.Winner } )
}
