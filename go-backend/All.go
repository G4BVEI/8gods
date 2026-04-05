package banana

import "fmt"

// permissive data-driven card game-engine
// for all the crazy cards the designer has made, check cartas.json
type (
	AllRooms struct {
		Rooms       []*Room
		SecretRooms map[string]*Room //string acting as password/name
	}
	Room struct {
		Title     string //will be ****** for SecretRooms
		Players   []*Player
		Active    bool
		Gamestate Gamestate
	}
	Gamestate struct {
		PlayerOrder  []*Player
		CurrentRound int
		ActivePlayer *Player
		HasPlayed    []*Player
		//A round is defined by a turn of each player(on normal conditions) so this array helps us maintain stuff going
		//this also permits for completely ramdom play order without unbalancing the game too much, (we only pool from who's not played already)
		// not that fairness is the main component but still
		EffectTimeline [][]TriggerAction
		//acts as a sliding window, index 0 always represent the current round
		//each [] as one round, for example [][][TriggerAction:["OnRoundStart" : "AttributeEffect(HasBeenSkipped) Source: *player Target *OtherPlayer]"]
		// so Player skipped SomePlayer 2 turns from now
		PermanentTriggers []TriggerAction
		// example TriggerAction[Trigger: OnAttackEvaluation, Action: DamageMultiply(*, 2)] doubles damage
		// or OnAttackEvaluation : ConvertElement(fire)
		PermanentEffects []Effect
		//like GameOrder(Shuffle) or Invert
		// maybe could be moved to above list
		ApocalipseHasStarted bool
	}
	TriggerAction struct {
		Trigger Trigger
		Action  Action
		Source  *Player
		Target  *Player //possibly will be an target type futurely like solo(one player, could include self) everyenemy(self explanatory) everyone(includes self)
	}
	Player struct {
		Websocket         *Websocket
		Name              string
		Life, Money, Mana int
		Hand              map[int]Card //int acting as card id, check dream curse as to why
		Miracles          []Card
		ActiveEffects     []Effect
	}
	Card struct {
		Id      int
		Name    string
		Actions []TriggerAction
		Tags    []CardTag
	}
	Card2 struct { //card necessarily have one nature but can have more than one
		Name    string
		Active  ActiveNature  //one of these AttackType, ProbAttackType, Consumable, Magic (all active turn cards)
		Defense DefenseNature //one of the following Reflect, Swingback or defense(all Defensive turn cards)
		Special SpecialNature // ondeath and oncarry cards ond so on(all passive turn cards)
	}
	AttackType struct {
		StackBase bool            // can be the first attack card to be played, eg a 2x damage card shouldnt do so whilst a +1 still could
		StackTop  bool            // can be stacked on top of other cards, only cards with extra effects should do so like +5 or 3x
		Damage    int             // very simple attack object internals 0
		Element   Element         //self explanatory
		Triggers  []TriggerAction // a list of secondary stuff the attack may do
	}
	DefenseType struct { // defense types always stack
		DamageDefended int
		Element        Element         //determines what it can or cant defend
		Triggers       []TriggerAction // a list of secondary stuff the attack may do
	}
	ProbType struct {
		Outcomes []Outcome // defines all the possibilities that can from that card
	}
	Outcome struct {
		Prob
		//aqui iria alguma nature
	}
	AreaAttackType struct {
		//this attack type always hits everyone but it generally has a high chance to miss(check unluck effect), everyone gets iterated dinamically making the gameflow interesting and chaotic
		Prob    int
		Damage  int
		Element Element
		Action  []TriggerAction
	}
	Trigger struct {
		any
	}
	Action struct {
		any
	}
	Effect struct {
		any
	}
	Websocket struct {
		any
	}
	CardTag struct {
		any
	}
	Element struct {
		any
	}
	FightPlayer interface { //premature interface just for basic thinkihg
		Hit([]Card) int
		TakeDamage(*Player, int) int
		GetInfo(*Player) (string, int)
		IsDeath(*Player) bool
	}
)

func main() {
	fmt.Print("banana")
}
