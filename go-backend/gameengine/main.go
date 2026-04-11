package main

import "reflect"

// ===================== CORE =====================
type (
	//AllRooms struct {
	//	Rooms       []*Channel
	//	SecretRooms map[string]*Channel //string acting as name
	//}
	Room struct {
		Players   []*Player
		Active    bool
		Gamestate Gamestate
	}
	Gamestate struct {
		PlayerOrder  []*Player
		ActivePlayer *Player
		//		HasPlayed            []*Player //if everyone has played we count a round pass
		//		CurrentRound         int
		//		EffectTimeline       [][]TriggerAction //index 0 always acts as the current round, its an sliding window for delayed card effects
		//		PermanentEffects     []Effect          // such as gameorder shuffled/inversed
		Triggers []TriggerAction // efeitos permanentes de mesa (termino-da-criacao)
		Cemetery []int           // registro de todas as cartas jogadas (passado, boneco-mimico)
		Store    []int           //acting as card id
		//		ApocalipseHasStarted bool
		UsedMiracles map[string]int //string as playername, int as cardid
		Ended        bool
	}
)

// ===================== PLAYER =====================

type Player struct {
	Connection *Connection

	Name  string
	Life  int
	Money int
	Mana  int
	Hand  []HandCard
	//int acting as id, if has dream use the value as true card
	// key is for what the player sees value for true card
	// if value = 0 card is real
	UsedMiracles   []int // miracles revelados — visíveis a todos após primeiro uso
	StatusEffects  []StatusEffect
	ActiveTriggers map[Trigger][]*Card
}

// ===================== CARD =====================
type HandCard struct {
	Card     Card
	RealCard Card //if nil(or 0) doesnt exist, only gets checked if the player has the dream effect
}

type Card struct {
	Id        int
	Name      string // besides logging i also dont need it
	IsMiracle bool
	Cost      int //if it is a miracle, price acts as mana cost to cast, else acts as buy price
	Rarity    int // wont be handled on this part but i prob gotta remember it
	//Tags      map[any]any //defines what the bot sees, and also defines behaviour instead of us preloading the cards
	// Uma carta pode ter Active + Defense simultaneamente.
	// O engine resolve qual nature usar baseado na fase do turno:
	// fase de ataque → Active, fase de defesa → Defense. Special -> passivo
	onAction   //ataques/consumiveis etc
	*onDefense //refletir, rebater, defender
	onPassive  // passivo /permanente, não depende de fase
}

// ActiveNature
type onAction interface {
	isActive()
}

func (AttackType) isActive()     {}
func (ProbAttackType) isActive() {}
func (Consumable) isActive()     {}

// DefenseNature

// Special Nature
type onPassive interface {
	isPassive()
}

func (PassiveType) isPassive() {}

type PassiveType struct {
	Behaviour []TriggerAction
}

type UsedMiracle struct {
	CardId int
	Owner  *Player
}

type AttackType struct {
	StackBase bool // can initiate an attack
	StackTop  bool // can be played on top of other cards
	Damage    int
	//	DamageSource DamageSource // overrides Damage quando non-nil
	//
	// Multiplicador aplicado após toda a avaliação do stack.
	// 0 = ignorado, 2.0 = dobra (aura-de-sansao).
	//
	//	DamageMultiplier float64
	//	Element          Element
	//
	// Quantas vezes repetir o ataque resolvido após o primeiro hit.
	// 0 = hit único, 5 = martelo-juizo-final (1 + 5 repetições).
	//
	//	RepeatCount int
	//	Triggers    []TriggerAction
} //btw we always use the target type of the target type and repeat count of the base

// DamageSource permite que cartas cujo dano é derivado de um stat do jogador
// no momento da resolução (bastao-do-ex = mana*2, sacolada = money*1).
// Se nil, Damage é usado diretamente.
type DamageSource struct {
	Stat       string  // "mana" | "money" | "life"
	Multiplier float64 // e.g. 2.0 para bastao-do-ex
	DrainStat  bool    // se true, zera o stat após resolver
}

// ===================== PROB / CHAOS =====================

// ProbAttackType representa um ataque com múltiplos resultados ponderados.
// Outcomes devem somar 100. Cada outcome pode ser qualquer ActiveNature,
// inclusive outro ProbAttackType (caos aninhado — válido por design).
type ProbAttackType struct {
	Outcomes []Outcome
}

type Outcome struct {
	Probability int
	onAction    // AttackType, ProbAttackType, Consumable, etc.
	Target      TargetSpec
}

// ===================== CONSUMABLE =====================

type Consumable struct {
	Actions []Action
}

// ===================== DEFENSE =====================

type DefenseType interface {
	isDefenseType()
}

func (ModifyType) isDefenseType() {}
func (ReduceType) isDefenseType() {}

type onDefense struct {
	Attack     DefenseType
	Miracle    DefenseType
	Consumable DefenseType
	AoE        DefenseType
	Curse      DefenseType
}

type ReduceType struct {
	DamageReduced int
	Element                       // tipo elemental deste card — determina quais ataques ele bloqueia
	Triggers      []TriggerAction // reações após levar dano (pecado-ganacioso, anel-de-mana, destruidor-da-luz)
}

type ModifyType struct {
	ReactionType
	Elements []Element //what element it can React to
}

type ReactionType int

const (
	block ReactionType = iota
	swingback
	deflect
)

// PermanentBonus é um exemplo de SpecialNature: enquanto este card está na mão
// (ou foi jogado permanentemente), modifica algum aspecto do jogo.
type PermanentBonus struct {
	Trigger Trigger
	Actions []Action
}

type Element int

const (
	ElementNormal Element = iota
	ElementDarkness
	ElementWater
	ElementFire
	ElementGround
	ElementPlant
	ElementLight
)

var elementName = map[Element]string{
	ElementNormal:   "Normal",
	ElementDarkness: "Darkness",
	ElementWater:    "Water",
	ElementFire:     "Fire",
	ElementGround:   "Ground",
	ElementPlant:    "Plant",
	ElementLight:    "Light",
}

type TriggerType int

const (
	onHit TriggerType = iota
	onDeath
	onRoundStart
	onTurnStart
	onTurnEnd
	onDefenseTurn
	onAttackTurn
	onAttackEvaluation
	onMiss
	onDefenseEvaluation
)

var expectedTriggerData = map[TriggerType]reflect.Type{
	onHit:               reflect.TypeOf([]*Player(nil)),
	onDeath:             reflect.TypeOf([]*Player(nil)),
	onRoundStart:        reflect.TypeOf(int32(0)),
	onTurnStart:         reflect.TypeOf([]*Player(nil)),
	onTurnEnd:           reflect.TypeOf([]*Player(nil)),
	onDefenseTurn:       reflect.TypeOf([]*Player(nil)),
	onAttackTurn:        reflect.TypeOf([]*Player(nil)),
	onAttackEvaluation:  reflect.TypeOf([]*Player(nil)),
	onMiss:              reflect.TypeOf([]*Player(nil)),
	onDefenseEvaluation: reflect.TypeOf([]*Player(nil)),
}

type Action struct {
	ActionType
	Data any
	TargetSpec
}
type ActionInstance struct {
	Action
	Targets []*Player
}
type TriggerAction struct {
	Condition Trigger
	Action    Action
}
type Trigger struct {
	TriggerType TriggerType
	Data        any
}

type ActionType int

const (
	attributeEffect ActionType = iota
	setProperties              // setProperties health: 8, money: 8,
	addProperties              //heal goes here as addProperties, health: 5
	addCardToStack
	ShowCard //probably better as a composable?
	StealCard
	getCardFromCemetery //by index or any/all
	refreshStore
	modifyStoreCount
	setStoreCount
	healCondition
)

type PropertyDelta struct {
	Health int16
	Money  int16
	Mana   int16
}
type NoData struct{}

var expectedActionData = map[ActionType]reflect.Type{
	attributeEffect:     reflect.TypeOf([]StatusEffect(nil)),
	setProperties:       reflect.TypeOf(PropertyDelta{}),
	addProperties:       reflect.TypeOf(PropertyDelta{}),
	addCardToStack:      reflect.TypeOf(int16(0)), //acting as id
	ShowCard:            reflect.TypeOf(int16(0)), //havent figured this one out
	StealCard:           reflect.TypeOf(int16(0)),
	getCardFromCemetery: reflect.TypeOf(GetCardFromCemetery{}),
	refreshStore:        reflect.TypeOf(NoData{}),
	modifyStoreCount:    reflect.TypeOf(int16(0)),
	setStoreCount:       reflect.TypeOf(int16(0)),
	healCondition:       reflect.TypeOf([]StatusEffect(nil)),
}

type GetCardFromCemetery struct {
	Mode  CemeteryMode
	index int
}

type CemeteryMode int

const (
	CemeteryAny CemeteryMode = iota
	CemeteryIndex
)

type TargetSpec struct {
	TargetType TargetType
	Quantity   int // if 0 q = 1
}

type TargetType int

const (
	randomAny TargetType = iota
	randomEnemy
	all
	allEnemy
	self
	chosen
)

type Effect int

type StatusEffect int

const (
	dream StatusEffect = iota
	unluck
	cold
	fever
	inferno
	heaven
	fog
	flash
)

type Connection struct {
	any
}
type Channel struct {
	any
}

//	func StackElements(a, b Element) Element {
//		if (a == "light" && b.Name == "darkness") || (a.Name == "darkness" && b.Name == "light") {
//			return ElementNormal
//		}
//		if a.Name == "light" {
//			return b
//		}
//		if b.Name == "light" {
//			return a
//		}
//		if a.Name == b.Name {
//			return a
//		}
//		return ElementNormal
//	}
func SortPlayers() {
}
func AwaitForPlay(*Player) {}
func gameloop() {
	SortPlayers()

}
func attackloop(ctx Gamestate) {
	for ctx.Ended == false { //check if the game has ended
		AwaitForPlay(ctx.ActivePlayer)
	}
	AwaitForEndGame()
}
func AwaitForEndGame() {}
