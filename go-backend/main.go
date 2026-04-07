package main

// ===================== CORE =====================

type (
	AllRooms struct {
		Rooms       []*Channel
		SecretRooms map[string]*Channel //string acting as name
	}
	Room struct {
		Players   []*Player
		Active    bool
		Gamestate Gamestate
	}
	Gamestate struct {
		PlayerOrder          []*Player
		ActivePlayer         *Player
		HasPlayed            []*Player //if everyone has played we count a round pass
		CurrentRound         int
		EffectTimeline       [][]TriggerAction //index 0 always acts as the current round, its an sliding window for delayed card effects
		PermanentEffects     []Effect          // such as gameorder shuffled/inversed
		PermanentTriggers    []TriggerAction   // efeitos permanentes de mesa (termino-da-criacao)
		PlayedCardLog        []int             // registro de todas as cartas jogadas (passado, boneco-mimico)
		Store                []int             //acting as card id
		ApocalipseHasStarted bool
	}
)

// ===================== PLAYER =====================

type Player struct {
	Connection    *Connection
	Name          string
	Life          int
	Money         int
	Mana          int
	Hand          map[int]int //int acting as id, tenho q pensar num jeito bão de fazer as carta falsa, if has dream use the value as true card (but dont tell the user)
	UsedMiracles  []int       // miracles revelados — visíveis a todos após primeiro uso
	ActiveEffects []StatusEffect
}

// ===================== CARD =====================

type Card struct {
	Name      string // besides logging i also dont need it
	IsMiracle bool   // this defines if the card gets deleted or not after play and if it cost mana to cast
	Cost      int    //if it is a miracle, price acts as mana cost, else acts as price
	Rarity    int    // wont be handled on this part but i prob gotta remember it
	// Uma carta pode ter Active + Defense simultaneamente.
	// O engine resolve qual nature usar baseado na fase do turno:
	// fase de ataque → Active, fase de defesa → Defense. Special -> passivo
	Active  ActiveNature  //ataques/consumiveis etc
	Defense DefenseNature //refletir, rebater, defender
	Special SpecialNature // passivo /permanente, não depende de fase
}

type ActiveNature struct {
	any
}
type DefenseNature struct {
	any
}
type SpecialNature struct {
	any
}

type UsedMiracle struct {
	CardId int
	Owner  *Player
}

type AttackType struct {
	StackBase    bool // can initiate an attack
	StackTop     bool // can be played on top of other cards
	Damage       int
	DamageSource DamageSource // overrides Damage quando non-nil
	// Multiplicador aplicado após toda a avaliação do stack.
	// 0 = ignorado, 2.0 = dobra (aura-de-sansao).
	DamageMultiplier float64
	Element          Element
	// Quantas vezes repetir o ataque resolvido após o primeiro hit.
	// 0 = hit único, 5 = martelo-juizo-final (1 + 5 repetições).
	RepeatCount int
	Triggers    []TriggerAction
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
	Nature      ActiveNature // AttackType, ProbAttackType, Consumable, etc.
	Target      TargetSpec
}

// ===================== CONSUMABLE =====================

type Consumable struct {
	Actions []ActionInstance
}

// ===================== DEFENSE =====================

// DefenseResponse descreve o que acontece quando este card intercepta algo.
// Block e Reflect podem ser true simultaneamente.

// DefenseCapability declara quais categorias de jogadas este card pode
// interceptar e como responde a cada uma. Zero value = não faz nada.

type DefenseType struct {
	DamageDefended int
	Element        Element         // tipo elemental deste card — determina quais ataques ele bloqueia
	Triggers       []TriggerAction // reações após levar dano (pecado-ganacioso, anel-de-mana, destruidor-da-luz)
}

type ReactType struct {
	//something here to represent either block, reflect or swingback
	Element Element //what element it is, if null = light
	//each of the following represent what this reaction applies to
	Attack     bool // nondeclared = false
	Miracle    bool // nondeclared = false
	Consumable bool // nondeclared = false
	AoE        bool // nondeclared = false
	Curse      bool // nondeclared = false
}

// PermanentBonus é um exemplo de SpecialNature: enquanto este card está na mão
// (ou foi jogado permanentemente), modifica algum aspecto do jogo.
type PermanentBonus struct {
	Trigger TriggerType
	Actions []ActionInstance
}

type Element struct {
	any
}

type TriggerType struct {
	any
}

type ActionInstance struct {
	any
}
type TriggerAction struct {
	Trigger Trigger
	Action  Action
	Source  *Player
	Target  TargetSpec
}
type Trigger struct {
	any
}
type Action struct {
	any
}
type TargetSpec struct {
	any
}
type Effect struct {
	any
}
type StatusEffect struct {
	any
}
type Connection struct {
	any
}
type Channel struct {
	any
}
