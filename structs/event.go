package structs

type Event struct {
	Type         string
	Format       string
	Participants []Player
	Pairings     []Pairing
	Date         string
	Store        string
	Bonus        string
}
