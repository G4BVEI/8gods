package main

import "fmt"
//permissive data-driven card game-engine
// for all the crazy cards the designer has made, check cartas.json
type(
AllRooms struct {
	Rooms []*Room
	SecretRooms map[string]*Room//string acting as password/name
}
Room struct{
	Title string//will be ****** for SecretRooms
	Players []Player
	Active bool
	Gamestate Gamestate
}
Gamestate struct{
	PlayerOrder []*Player
	CurrentRound int
	ActivePlayer *Player
	HasPlayed []*Player
	//A round is defined by a turn of each player(on normal conditions) so this array helps us maintain stuff going
	//this also permits for completely ramdom play order without unbalancing the game too much, (we only pool from who's not played already)
	// not that fairness is the main component but still
	EffectTimeline []map[Trigger]Action
	//each [] as one round, for example [][]["OnRoundStart" : "AttributeEffect(HasBeenSkipped, SomePlayer))"]
	// so SomePlayer will be skipped 2 turns from now
	PermanentTriggers map[Trigger]Action
	// example OnAttackEvaluation: DamageMultiply(*, 2)) doubles damage
	// or OnAttackEvaluation : ConvertElement(fire)
	PermanentEffects []Effect
	//like GameOrder(Shuffle) or Invert
	// maybe could be moved to above list
}
Player struct {
	Websocket *Websocket
	Name string
	Life, Money, Mana int
	Hand []Card
	ActiveEffects []Effect
}
AttackCard struct {
	StackBase bool // can be the first attack card to be played, eg a 2x damage card shouldnt do so
	StackTop bool // can be stacked on top of other cards, only cards with extra effects should do so
	Attack Attack // the attack itself
	Triggers map[Trigger]Action// a map of all the effects with its triggers so we can mantain this all more easily and make a priority list
}
ProbAttackCard struct {
	StackBase bool // can be the first attack card to be played, eg a 2x damage card shouldnt do so
	StackTop bool // can be stacked on top of other cards, only cards with extra effects should do so
	Outcomes []Outcome // defines all the possibilities that can from that card
	//passar função outcome ao inves de outcomes array?
}
Outcome struct {
	Probabily int //prob this effect does get passed
	Attack Attack // the attack itself(next struct)
	Triggers map[Trigger]Action// a map of all the effects with its triggers so we can mantain this all more easily and make a priority list
}
Attack struct {
	Damage int // very simple attack object internals
	Element Element
}
Trigger struct{
	any
}
Action struct{
	any
}
Effect struct{
	any
}
Websocket struct{
	any
}
Element struct{
	any
}
FightPlayer interface {
   Hit([]AttackCard) int
   TakeDamage(*Player int) int
   GetInfo(*Player) (string, int)
   IsDeath(*Player) bool
}
)

func main(){
	fmt.Print("banana")
}
