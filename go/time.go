type Gameloop struct{
	PlayerOrder []*Player
	Timeline []map[Trigger]Effect
	ActivePlayer *Player
}
