package structs

type Event struct {
	Type         string
	Format       string
	Participants []Player
	Pairings     []Pairing
	Store        string
}
