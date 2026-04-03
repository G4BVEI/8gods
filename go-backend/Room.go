type Rooms struct {
	Rooms []*Room
	PrivateRooms map[string]*Room//string acting as name
}
type Room struct{
	Players []Player
	Gameloop Gameloop
}
