package main

import "fmt"

// ===================== CORE =====================

type (
	AllRooms struct {
		Rooms       []*Room
		SecretRooms map[string]*Room
	}
	Room struct {
		Title     string
		Players   []*Player
		Active    bool
		Gamestate Gamestate
	}
	Gamestate struct {
		PlayerOrder          []*Player
		CurrentRound         int
		ActivePlayer         *Player
		HasPlayed            []*Player
		EffectTimeline       [][]TriggerAction
		PermanentEffects     []Effect
		PlayedCardLog        []Card          // registro de todas as cartas jogadas (passado, boneco-mimico)
		PermanentTriggers    []TriggerAction // efeitos permanentes de mesa (termino-da-criacao)
		ApocalipseHasStarted bool
	}
)

// ===================== PLAYER =====================

type Player struct {
	Websocket     *Websocket
	Name          string
	Life          int
	Money         int
	Mana          int
	Hand          map[int]Card
	UsedMiracles  []UsedMiracle // miracles revelados — visíveis a todos após primeiro uso
	ActiveEffects []StatusEffect
}

// ===================== CARD =====================

type Card struct {
	Id     int
	Name   string
	Price  int
	Rarity int
	// Uma carta pode ter Active + Defense simultaneamente.
	// O engine resolve qual nature usar baseado na fase do turno:
	// fase de ataque → Active, fase de defesa → Defense. Sempre.
	Active  ActiveNature
	Defense DefenseNature
	Special SpecialNature // passivo/permanente, não depende de fase
	Miracle *MiracleData  // non-nil apenas para cartas milagre
	Tags    []CardTag
}

// ===================== MIRACLE =====================

// MiracleData representa um milagre.
// Milagres ficam na mão e ficam visíveis a todos após serem usados pela primeira vez.
// Apenas um de Active ou Defense deve estar preenchido.
type MiracleData struct {
	ManaCost int
	Active   ActiveNature
	Defense  DefenseNature
}

type UsedMiracle struct {
	SourceCard Card
	Owner      *Player
}

// ===================== NATURE INTERFACES =====================

type ActiveNature interface {
	isActive()
}

type DefenseNature interface {
	isDefense()
}

// SpecialNature é para efeitos passivos/permanentes que não são jogadas ativas
// nem respostas de defesa — e.g. auras, bônus permanentes, regras alteradas.
type SpecialNature interface {
	isSpecial()
}

// ===================== ATTACK =====================

// DamageSource permite que cartas cujo dano é derivado de um stat do jogador
// no momento da resolução (bastao-do-ex = mana*2, sacolada = money*1).
// Se nil, Damage é usado diretamente.
type DamageSource struct {
	Stat       string  // "mana" | "money" | "life"
	Multiplier float64 // e.g. 2.0 para bastao-do-ex
	DrainStat  bool    // se true, zera o stat após resolver
}

type AttackType struct {
	StackBase    bool
	StackTop     bool
	Damage       int
	DamageSource *DamageSource // overrides Damage quando non-nil
	// Multiplicador aplicado após toda a avaliação do stack.
	// 0 = ignorado, 2.0 = dobra (aura-de-sansao).
	DamageMultiplier float64
	Element          Element
	// Quantas vezes repetir o ataque resolvido após o primeiro hit.
	// 0 = hit único, 5 = martelo-juizo-final (1 + 5 repetições).
	RepeatCount int
	Triggers    []TriggerAction
}

func (AttackType) isActive() {}

// ===================== PROB / CHAOS =====================

// ProbAttackType representa um ataque com múltiplos resultados ponderados.
// Outcomes devem somar 100. Cada outcome pode ser qualquer ActiveNature,
// inclusive outro ProbAttackType (caos aninhado — válido por design).
type ProbAttackType struct {
	Outcomes []Outcome
}

func (ProbAttackType) isActive() {}

type Outcome struct {
	Probability int
	Nature      ActiveNature // AttackType, ProbAttackType, Consumable, etc.
	Target      TargetSpec
}

// ===================== CONSUMABLE =====================

type Consumable struct {
	Actions []ActionInstance
}

func (Consumable) isActive() {}

// ===================== DEFENSE =====================

// DefenseResponse descreve o que acontece quando este card intercepta algo.
// Block e Reflect podem ser true simultaneamente.
type DefenseResponse struct {
	Block   bool
	Reflect bool
}

// DefenseCapability declara quais categorias de jogadas este card pode
// interceptar e como responde a cada uma. Zero value = não faz nada.
type DefenseCapability struct {
	Attack     DefenseResponse
	Miracle    DefenseResponse
	Consumable DefenseResponse
	AoE        DefenseResponse
	Curse      DefenseResponse
}

type DefenseType struct {
	DamageDefended int
	Element        Element // tipo elemental deste card — determina quais ataques ele bloqueia
	Capability     DefenseCapability
	Triggers       []TriggerAction // reações após levar dano (pecado-ganacioso, anel-de-mana, destruidor-da-luz)
}

func (DefenseType) isDefense() {}

// ===================== SPECIAL / PASSIVE =====================

// PermanentBonus é um exemplo de SpecialNature: enquanto este card está na mão
// (ou foi jogado permanentemente), modifica algum aspecto do jogo.
type PermanentBonus struct {
	Actions []ActionInstance
	Trigger TriggerType
}

func (PermanentBonus) isSpecial() {}

// ===================== STATUS EFFECT =====================

// StatusEffect rastreia uma maldição ou efeito temporário ativo num jogador.
// Data carrega payload arbitrário para efeitos que precisam de estado:
//   - ExtraTurns: int                        (tempo-relativo)
//   - BombCard: Card                         (bomba-do-destino — portador atual)
//   - SkipRounds: int                        (pular N rodadas)
type StatusEffect struct {
	Kind       CurseKind
	RoundsLeft int // -1 = permanente até ser curado
	Data       any // payload opcional por tipo de efeito
}

type CurseKind string

const (
	CurseCold   CurseKind = "cold"
	CurseFever  CurseKind = "fever"
	CurseHell   CurseKind = "hell"
	CurseHeaven CurseKind = "heaven"
	CurseFog    CurseKind = "fog"
	CurseFlash  CurseKind = "flash"
	CurseDream  CurseKind = "dream"  // dono vê própria mão como 50% falsa até jogar
	CurseUnluck CurseKind = "unluck" // ataques % sempre acertam
	// efeitos de jogo armazenados como StatusEffect
	EffectExtraTurns CurseKind = "extra_turns" // Data: int — quantos turnos extras restam
	EffectBomb       CurseKind = "bomb"        // Data: Card — a bomba-do-destino em si
	EffectSkip       CurseKind = "skip"        // Data: int — rodadas a pular
)

// WorsenDisease retorna o próximo estágio da doença se existir.
func WorsenDisease(c CurseKind) (CurseKind, bool) {
	chain := map[CurseKind]CurseKind{
		CurseCold:  CurseFever,
		CurseFever: CurseHell,
		CurseHell:  CurseHeaven,
	}
	next, ok := chain[c]
	return next, ok
}

// ===================== TRIGGERS =====================

type TriggerAction struct {
	Trigger TriggerType
	Target  TargetSpec
	Actions []ActionInstance
	Delayed int // rodadas de delay; 0 = imediato
}

type TriggerType string

const (
	TrigOnRoundStart TriggerType = "on_round_start"
	TrigOnRoundEnd   TriggerType = "on_round_end"
	TrigOnTurnStart  TriggerType = "on_turn_start"
	TrigOnTurnEnd    TriggerType = "on_turn_end"

	TrigOnAttackDeclared TriggerType = "on_attack_declared"
	TrigOnAttackEval     TriggerType = "on_attack_evaluation" // multiplicadores, termino-da-criacao
	TrigOnAttackResolved TriggerType = "on_attack_resolved"

	TrigOnDefenseDeclared TriggerType = "on_defense_declared"
	TrigOnDefenseEval     TriggerType = "on_defense_evaluation"
	TrigOnDefenseResolved TriggerType = "on_defense_resolved"

	TrigOnDamageTaken TriggerType = "on_damage_taken"
	TrigOnDamageDealt TriggerType = "on_damage_dealt"

	TrigOnDeath  TriggerType = "on_death" // amuleto-da-vida-eterna, renacimento, boneco-vingativo
	TrigOnRevive TriggerType = "on_revive"
	TrigOnSkip   TriggerType = "on_skip"

	TrigOnCardPlayed    TriggerType = "on_card_played"
	TrigOnCardDiscarded TriggerType = "on_card_discarded"
	TrigOnCardBought    TriggerType = "on_card_bought"

	TrigOnCurseApplied TriggerType = "on_curse_applied"
	TrigOnCurseWorsen  TriggerType = "on_curse_worsen"
	TrigOnCurseRemoved TriggerType = "on_curse_removed"

	TrigOnStorePurchase TriggerType = "on_store_purchase"
	TrigOnStoreRefresh  TriggerType = "on_store_refresh"

	TrigOnOrderShuffle TriggerType = "on_order_shuffle"
	TrigOnOrderInvert  TriggerType = "on_order_invert"
)

// ===================== ACTION SYSTEM =====================

type ActionInstance struct {
	Type   string
	Value  any
	Target TargetSpec
}

// ===================== TARGET =====================

type TargetSpec struct {
	Type  TargetType
	Count int // número de alvos; 0 = padrão (1 alvo), -1 = todos
}

type TargetType string

const (
	TargetSelf        TargetType = "self"
	TargetAll         TargetType = "all"
	TargetRandom      TargetType = "random"
	TargetRandomEnemy TargetType = "random_enemy"
	TargetChosen      TargetType = "chosen"
	TargetEnemies     TargetType = "enemies"
)

// ===================== ELEMENT =====================

type Element struct {
	Name string
}

var (
	ElementNormal   = Element{"normal"}
	ElementDarkness = Element{"darkness"}
	ElementWater    = Element{"water"}
	ElementFire     = Element{"fire"}
	ElementGround   = Element{"ground"}
	ElementPlant    = Element{"plant"}
	ElementLight    = Element{"light"}
)

// CanDefendWith retorna true quando um card de defesa do tipo def pode bloquear
// um ataque do tipo att.
func CanDefendWith(att, def Element) bool {
	switch att.Name {
	case "normal", "darkness":
		return true
	case "water":
		return def.Name == "fire"
	case "fire":
		return def.Name == "water"
	case "ground":
		return def.Name == "plant"
	case "plant":
		return def.Name == "ground"
	case "light":
		return def.Name == "light"
	}
	return false
}

// StackElements resolve o elemento resultante quando duas camadas de ataque
// são empilhadas.
func StackElements(a, b Element) Element {
	if (a.Name == "light" && b.Name == "darkness") || (a.Name == "darkness" && b.Name == "light") {
		return ElementNormal
	}
	if a.Name == "light" {
		return b
	}
	if b.Name == "light" {
		return a
	}
	if a.Name == b.Name {
		return a
	}
	return ElementNormal
}

// ===================== STORE =====================

type StoreCard struct {
	Card
	DisplayPrice int // preço atual; igual a Card.Price quando não afetado por efeitos
}

type Store struct {
	Display []StoreCard
}

// ===================== BASIC TYPES =====================

type CardTag struct {
	Name string
}

type Effect struct {
	Type  string
	Value any
}

type Websocket struct {
	any
}

// ===================== MAIN =====================

func main() {
	fmt.Println("engine ready")
}
