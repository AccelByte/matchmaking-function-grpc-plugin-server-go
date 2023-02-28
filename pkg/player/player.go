package player

// PlayerData is an id for a player along with a collection of arbitrary data used for matching this player with others
type PlayerData struct {
	PlayerID   ID
	PartyID    string
	Attributes map[string]interface{}
}

// ID is a unique identifier for a player
type ID string

// IDToString returns a string representation of the player id
func IDToString(u ID) string {
	return string(u)
}

// IDFromString converts a string to a player.ID
func IDFromString(u string) ID {
	return ID(u)
}

func ToID(p PlayerData) ID {
	return p.PlayerID
}
