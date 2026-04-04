package main

import (
	"fmt"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Element types
type Element string

const (
	Normal   Element = "normal"
	Darkness Element = "darkness"
	Water    Element = "water"
	Fire     Element = "fire"
	Rock     Element = "rock"
	Wood     Element = "wood"
	Light    Element = "light"
)

// Trigger types for card effects
type Trigger string

const (
	OnPlay          Trigger = "on_play"
	OnAttack        Trigger = "on_attack"
	OnDefend        Trigger = "on_defend"
	OnDamage        Trigger = "on_damage"
	OnDeath         Trigger = "on_death"
	OnTurnStart     Trigger = "on_turn_start"
	OnTurnEnd       Trigger = "on_turn_end"
	OnRoundStart    Trigger = "on_round_start"
	OnRoundEnd      Trigger = "on_round_end"
	OnBuy           Trigger = "on_buy"
	OnDiscard       Trigger = "on_discard"
	OnAttackEval    Trigger = "on_attack_evaluation"
	OnDefenseEval   Trigger = "on_defense_evaluation"
)

// Curse/Disease types
type Curse string

const (
	Cold   Curse = "cold"
	Fever  Curse = "fever"
	Hell   Curse = "hell"
	Heaven Curse = "heaven"
	Fog    Curse = "fog"
	Flash  Curse = "flash"
	Dream  Curse = "dream"
	DarkCloud Curse = "dark_cloud"
)

type Action interface{}
type Effect interface{}

// Action types
type DamageAction struct {
	Amount  int
	Element Element
	Target  *Player
}

type HealAction struct {
	Amount int
	Target *Player
}

type GainManaAction struct {
	Amount int
	Target *Player
}

type GainMoneyAction struct {
	Amount int
	Target *Player
}

type ApplyCurseAction struct {
	Curse  Curse
	Target *Player
}

type SkipTurnAction struct {
	Target     *Player
	RoundsSkip int // how many rounds to skip
}

type DrawCardAction struct {
	Target   *Player
	Count    int
	FromDeck string // "attack" or "secondary"
}

type DiscardCardAction struct {
	Target  *Player
	CardID  string
}

type ShuffleOrderAction struct{}

type ReflectAction struct {
	BackToAttacker bool
}

type SwingAction struct {
	RandomTarget bool
}

type StoreModifyAction struct {
	AddSlots    int
	Refresh     bool
	DiscountAll int
}

type ConvertDamageAction struct {
	ToElement Element
	Multiplier float64
}

type ReviveAction struct {
	HP    int
	Mana  int
	Money int
}

type WebSocket struct {
	// WebSocket connection fields would go here
	ID string
}

type Card struct {
	ID          string
	Name        string
	Img         string
	Description string

	// Card can have multiple types
	IsAttack     bool
	IsDefense    bool
	IsMiracle    bool
	IsConsumable bool
	IsSpecial    bool

	// Attack properties
	AttackType   *AttackType
	ProbAttack   *ProbAttackType

	// Defense properties
	DefenseType  *DefenseType

	// Common properties
	Rarity       string
	Price        int
	ManaCost     int
	Triggers     map[Trigger]Action
}

type AttackType struct {
	StackBase    bool // Can be first attack
	StackTop     bool // Can stack on top
	Damage       int
	Element      Element
	IsAOE        bool
	Triggers     map[Trigger]Action
}

type DefenseType struct {
	Defense      int
	Element      Element
	Triggers     map[Trigger]Action
}

type ProbAttackType struct {
	Outcomes     []Outcome
}

type Outcome struct {
	Probability  int // 0-100
	Damage       int
	Element      Element
	Triggers     map[Trigger]Action
}

type Player struct {
	WebSocket    *WebSocket
	Name         string
	Life         int
	Money        int
	Mana         int
	Hand         []Card
	AttackDeck   []Card
	SecondaryDeck []Card
	DiscardPile  []Card
	ActiveEffects []Effect
	Curses       map[Curse]int // curse -> severity level
	HasPlayedThisRound bool
	IsSkipped     bool
	SkippedRounds int
}

type Store struct {
	DisplayCards []Card
	BaseSlots    int
}

type Room struct {
	Title       string
	Players     []*Player
	Active      bool
	GameState   GameState
}

type GameState struct {
	PlayerOrder       []*Player
	CurrentRound      int
	CurrentTurnIndex  int
	ActivePlayer      *Player
	HasPlayed         []*Player
	EffectTimeline    []map[Trigger]Action
	PermanentTriggers map[Trigger]Action
	PermanentEffects  []Effect
	AwaitingDefense   *DefenseContext
	Store       Store
	AttackDeck  []Card
	SecondaryDeck []Card
	DiscardPile []Card
	ApocalypseActive bool
}

type DefenseContext struct {
	Attacker        *Player
	Defender        *Player
	OriginalDamage  int
	OriginalElement Element
	StackedAttacks  []Card
}

type GameLoop struct {
	Room *Room
}

// Core Game Functions

func NewRoom(title string) *Room {
	return &Room{
		Title:       title,
		Players:     make([]*Player, 0),
		Active:      true,
		Store: Store{
			DisplayCards: make([]Card, 2),
			BaseSlots:    2,
		},
		AttackDeck:      make([]Card, 0),
		SecondaryDeck:   make([]Card, 0),
		DiscardPile:     make([]Card, 0),
		ApocalypseActive: false,
		GameState: GameState{
			PlayerOrder:       make([]*Player, 0),
			HasPlayed:         make([]*Player, 0),
			EffectTimeline:    make([]map[Trigger]Action, 0),
			PermanentTriggers: make(map[Trigger]Action),
			PermanentEffects:  make([]Effect, 0),
		},
	}
}

func (r *Room) AddPlayer(name string) error {
	if len(r.Players) >= 8 {
		return fmt.Errorf("room is full (max 8 players)")
	}

	player := &Player{
		Name:         name,
		Life:         40,
		Mana:         10,
		Money:        15,
		Hand:         make([]Card, 0),
		ActiveEffects: make([]Effect, 0),
		Curses:       make(map[Curse]int),
	}

	r.Players = append(r.Players, player)
	return nil
}

func (r *Room) StartGame() error {
	if len(r.Players) < 2 {
		return fmt.Errorf("need at least 2 players to start")
	}

	// Initialize decks (you'll need to load cards from JSON)
	r.InitializeDecks()

	// Shuffle decks
	r.ShuffleDecks()

	// Deal initial cards
	for _, player := range r.Players {
		// Draw 4 attack cards and 4 secondary cards
		for i := 0; i < 4; i++ {
			player.DrawCard(r, "attack")
			player.DrawCard(r, "secondary")
		}
	}

	// Randomize player order
	r.RandomizePlayerOrder()

	// Start first round
	r.GameState.CurrentRound = 1
	r.GameState.CurrentTurnIndex = 0
	r.GameState.ActivePlayer = r.GameState.PlayerOrder[0]

	return nil
}

func (r *Room) InitializeDecks() {
	// This would load from your JSON file
	// For now, create placeholder decks
	r.AttackDeck = make([]Card, 0)
	r.SecondaryDeck = make([]Card, 0)
}

func (r *Room) ShuffleDecks() {
	// Shuffle logic
	rand.Shuffle(len(r.AttackDeck), func(i, j int) {
		r.AttackDeck[i], r.AttackDeck[j] = r.AttackDeck[j], r.AttackDeck[i]
	})
	rand.Shuffle(len(r.SecondaryDeck), func(i, j int) {
		r.SecondaryDeck[i], r.SecondaryDeck[j] = r.SecondaryDeck[j], r.SecondaryDeck[i]
	})
}

func (r *Room) RandomizePlayerOrder() {
	r.GameState.PlayerOrder = make([]*Player, len(r.Players))
	copy(r.GameState.PlayerOrder, r.Players)
	rand.Shuffle(len(r.GameState.PlayerOrder), func(i, j int) {
		r.GameState.PlayerOrder[i], r.GameState.PlayerOrder[j] = r.GameState.PlayerOrder[j], r.GameState.PlayerOrder[i]
	})
}

func (p *Player) DrawCard(room *Room, deckType string) {
	var card Card
	var fromDeck *[]Card

	if deckType == "attack" {
		fromDeck = &room.AttackDeck
	} else {
		fromDeck = &room.SecondaryDeck
	}

	if len(*fromDeck) == 0 {
		// Reshuffle discard pile
		*fromDeck = room.DiscardPile
		room.DiscardPile = make([]Card, 0)
		rand.Shuffle(len(*fromDeck), func(i, j int) {
			(*fromDeck)[i], (*fromDeck)[j] = (*fromDeck)[j], (*fromDeck)[i]
		})
	}

	if len(*fromDeck) > 0 {
		card = (*fromDeck)[0]
		*fromDeck = (*fromDeck)[1:]
		p.Hand = append(p.Hand, card)
	}
}

func (r *Room) ProcessTurn(player *Player, playedCard Card, additionalCards []Card) error {
	// Check if player is skipped
	if player.IsSkipped {
		player.IsSkipped = false
		if player.SkippedRounds > 0 {
			player.SkippedRounds--
			if player.SkippedRounds > 0 {
				player.IsSkipped = true
			}
		}
		r.NextTurn()
		return nil
	}

	// Process card based on type
	if playedCard.IsAttack || playedCard.AttackType != nil {
		return r.ProcessAttack(player, playedCard, additionalCards)
	} else if playedCard.IsDefense || playedCard.DefenseType != nil {
		// Defense cards are usually played in response, not as main action
		return fmt.Errorf("defense cards cannot be played as main action")
	} else if playedCard.IsMiracle {
		return r.ProcessMiracle(player, playedCard)
	} else if playedCard.IsConsumable {
		return r.ProcessConsumable(player, playedCard)
	}

	// Optional: Discard and buy
	// (This would be handled by separate game actions)

	player.HasPlayedThisRound = true
	r.GameState.HasPlayed = append(r.GameState.HasPlayed, player)

	// Process end of turn effects
	r.ProcessTriggers(OnTurnEnd, player)

	r.NextTurn()
	return nil
}

func (r *Room) ProcessAttack(attacker *Player, baseCard Card, stackedCards []Card) error {
	// Calculate total damage and element
	totalDamage := baseCard.AttackType.Damage
	currentElement := baseCard.AttackType.Element

	for _, card := range stackedCards {
		if card.AttackType != nil && card.AttackType.StackTop {
			totalDamage += card.AttackType.Damage
			currentElement = r.CombineElements(currentElement, card.AttackType.Element)
		}
	}

	// Determine target(s)
	var targets []*Player
	if baseCard.AttackType.IsAOE {
		targets = r.GetAllPlayersExcept(attacker)
	} else {
		targets = []*Player{r.SelectTarget(attacker)}
	}

	// Process attack for each target
	for _, target := range targets {
		// Apply attack modifiers from permanent effects
		totalDamage, currentElement = r.ApplyAttackModifiers(totalDamage, currentElement)

		// Create defense context
		defenseCtx := &DefenseContext{
			Attacker:        attacker,
			Defender:        target,
			OriginalDamage:  totalDamage,
			OriginalElement: currentElement,
			StackedAttacks:  append([]Card{baseCard}, stackedCards...),
		}

		// Give defender chance to defend
		r.GameState.AwaitingDefense = defenseCtx

		// Process defense (this would be handled by the defender's response)
		// For now, apply damage directly
		r.ApplyDamage(target, totalDamage, currentElement, attacker)
	}

	return nil
}

func (r *Room) ProcessDefense(defender *Player, defenseCards []Card, defenseCtx *DefenseContext) {
	totalDefense := 0
	defenseElement := Normal

	// Check for reflect or swing
	for _, card := range defenseCards {
		if card.DefenseType != nil {
			// Check if defense is valid against attack element
			if r.CanDefendAgainst(defenseCtx.OriginalElement, card.DefenseType.Element) {
				totalDefense += card.DefenseType.Defense
				defenseElement = card.DefenseType.Element
			}
		}

		// Check for special effects like reflect
		if card.Name == "espelho" {
			// Reflect everything
			r.ApplyDamage(defenseCtx.Attacker, defenseCtx.OriginalDamage, defenseCtx.OriginalElement, defender)
			return
		}

		// Check for swing
		if card.Name == "reflexo-do-heroi" {
			// Swing to random player
			randomTarget := r.GetRandomPlayerExcept(defender, defenseCtx.Attacker)
			if randomTarget != nil {
				r.ApplyDamage(randomTarget, defenseCtx.OriginalDamage/2, defenseCtx.OriginalElement, defender)
			}
		}
	}

	// Apply defense
	finalDamage := defenseCtx.OriginalDamage - totalDefense
	if finalDamage < 0 {
		finalDamage = 0
	}

	// Check for darkness instakill
	if defenseCtx.OriginalElement == Darkness && totalDefense < defenseCtx.OriginalDamage {
		finalDamage = defenseCtx.OriginalDamage // Full damage, no defense
	}

	if finalDamage > 0 {
		r.ApplyDamage(defender, finalDamage, defenseCtx.OriginalElement, defenseCtx.Attacker)
	}

	// Process defense triggers
	for _, card := range defenseCards {
		if card.DefenseType != nil && card.DefenseType.Triggers != nil {
			r.ProcessDefenseTriggers(card.DefenseType.Triggers, defender, defenseCtx)
		}
	}
}

func (r *Room) ApplyDamage(target *Player, damage int, element Element, source *Player) {
	// Apply damage modifiers
	for _, effect := range r.GameState.PermanentEffects {
		if modifier, ok := effect.(ConvertDamageAction); ok {
			if modifier.ToElement != "" {
				element = modifier.ToElement
			}
			if modifier.Multiplier > 0 {
				damage = int(float64(damage) * modifier.Multiplier)
			}
		}
	}

	// Apply curse effects
	if target.Curses[DarkCloud] > 0 {
		// Received attacks hit certainly
		// Damage is guaranteed
	}

	// Apply damage
	target.Life -= damage

	// Process on-damage triggers
	r.ProcessTriggers(OnDamage, target)

	// Check for death
	if target.Life <= 0 {
		r.ProcessDeath(target)
	}

	// Allow store purchase opportunity for non-AoE attacks
	// (This would be handled by the game UI)
}

func (r *Room) ProcessDeath(player *Player) {
	// Check for revive effects
	for _, effect := range player.ActiveEffects {
		if revive, ok := effect.(ReviveAction); ok {
			player.Life = revive.HP
			player.Mana = revive.Mana
			player.Money = revive.Money
			// Remove revive effect after use
			player.ActiveEffects = r.RemoveEffectFromSlice(player.ActiveEffects, effect)
			return
		}
	}

	// Remove player from game
	r.RemovePlayer(player)
}

func (r *Room) ProcessMiracle(player *Player, card Card) error {
	// Check mana cost
	if player.Mana < card.ManaCost {
		return fmt.Errorf("insufficient mana")
	}

	player.Mana -= card.ManaCost

	// Process miracle effects based on card
	// This would be card-specific logic
	r.ProcessCardTriggers(card, player)

	return nil
}

func (r *Room) ProcessConsumable(player *Player, card Card) error {
	// Process consumable effects
	r.ProcessCardTriggers(card, player)

	// Consumables are discarded after use
	player.RemoveCardFromHand(card.ID)

	return nil
}

func (r *Room) ProcessCardTriggers(card Card, player *Player) {
	if card.Triggers != nil {
		for trigger, action := range card.Triggers {
			if trigger == OnPlay {
				r.ExecuteAction(action, player)
			}
		}
	}
}

func (r *Room) ExecuteAction(action Action, player *Player) {
	switch a := action.(type) {
	case DamageAction:
		if a.Target != nil {
			r.ApplyDamage(a.Target, a.Amount, a.Element, player)
		}
	case HealAction:
		if a.Target != nil {
			a.Target.Life += a.Amount
		}
	case GainManaAction:
		if a.Target != nil {
			a.Target.Mana += a.Amount
		}
	case GainMoneyAction:
		if a.Target != nil {
			a.Target.Money += a.Amount
		}
	case ApplyCurseAction:
		if a.Target != nil {
			a.Target.Curses[a.Curse]++
		}
	case SkipTurnAction:
		if a.Target != nil {
			a.Target.IsSkipped = true
			a.Target.SkippedRounds = a.RoundsSkip
		}
	case DrawCardAction:
		if a.Target != nil {
			for i := 0; i < a.Count; i++ {
				a.Target.DrawCard(r, a.FromDeck)
			}
		}
	case ReviveAction:
		// Handled in ProcessDeath
	default:
		fmt.Printf("Unknown action type: %T\n", a)
	}
}

func (r *Room) ProcessTriggers(trigger Trigger, player *Player) {
	// Check permanent triggers
	if action, exists := r.GameState.PermanentTriggers[trigger]; exists {
		r.ExecuteAction(action, player)
	}

	// Check timeline triggers (for delayed effects)
	for _, timelineMap := range r.GameState.EffectTimeline {
		if action, exists := timelineMap[trigger]; exists {
			r.ExecuteAction(action, player)
		}
	}

	// Check player-specific triggers
	for _, effect := range player.ActiveEffects {
		// Process effect triggers
	}
}

func (r *Room) ProcessDiseaseProgression() {
	for _, player := range r.Players {
		for curse, level := range player.Curses {
			// 5% chance to worsen each turn
			if rand.Intn(100) < 5 {
				switch curse {
				case Cold:
					player.Curses[Cold] = 0
					player.Curses[Fever] = level
				case Fever:
					player.Curses[Fever] = 0
					player.Curses[Hell] = level
				case Hell:
					player.Curses[Hell] = 0
					player.Curses[Heaven] = level
				case Heaven:
					player.Curses[Heaven] = 0
					// Heaven to Hell transformation?
					if level > 0 {
						player.Curses[Hell] = level
					}
				}
			}
		}

		// Apply curse damage/healing
		if player.Curses[Cold] > 0 {
			player.Life -= 1
		}
		if player.Curses[Fever] > 0 {
			player.Life -= 2
		}
		if player.Curses[Hell] > 0 {
			player.Life -= 5
		}
		if player.Curses[Heaven] > 0 {
			player.Life += 5
		}
	}
}

func (r *Room) NextTurn() {
	// Move to next player
	r.GameState.CurrentTurnIndex++

	// Check if round is complete
	if r.GameState.CurrentTurnIndex >= len(r.GameState.PlayerOrder) {
		r.EndRound()
		return
	}

	r.GameState.ActivePlayer = r.GameState.PlayerOrder[r.GameState.CurrentTurnIndex]

	// Process turn start triggers
	r.ProcessTriggers(OnTurnStart, r.GameState.ActivePlayer)

	// Apply disease progression
	r.ProcessDiseaseProgression()
}

func (r *Room) EndRound() {
	// Process round end triggers
	for _, player := range r.Players {
		r.ProcessTriggers(OnRoundEnd, player)
		player.HasPlayedThisRound = false
	}

	// Reset for next round
	r.GameState.CurrentRound++
	r.GameState.CurrentTurnIndex = 0
	r.GameState.HasPlayed = make([]*Player, 0)
	r.GameState.ActivePlayer = r.GameState.PlayerOrder[0]

	// Process round start triggers
	for _, player := range r.Players {
		r.ProcessTriggers(OnRoundStart, player)
	}

	// Check win condition
	activePlayers := r.GetActivePlayers()
	if len(activePlayers) == 1 {
		r.EndGame(activePlayers[0])
	}
}

func (r *Room) EndGame(winner *Player) {
	r.Active = false
	fmt.Printf("Game Over! Winner: %s\n", winner.Name)
	// Broadcast win to all players
}

func (r *Room) CombineElements(e1, e2 Element) Element {
	// Light + anything but darkness = that element
	if e1 == Light && e2 != Darkness {
		return e2
	}
	if e2 == Light && e1 != Darkness {
		return e1
	}

	// Light + dark = normal
	if (e1 == Light && e2 == Darkness) || (e1 == Darkness && e2 == Light) {
		return Normal
	}

	// Element + same element = same element
	if e1 == e2 {
		return e1
	}

	// Different non-light elements = normal
	if e1 != Light && e2 != Light {
		return Normal
	}

	return Normal
}

func (r *Room) CanDefendAgainst(attackElement, defenseElement Element) bool {
	switch attackElement {
	case Water:
		return defenseElement == Fire
	case Fire:
		return defenseElement == Water
	case Rock:
		return defenseElement == Wood
	case Wood:
		return defenseElement == Rock
	case Light:
		// Light can sometimes be defended with luck
		return rand.Intn(100) < 20 // 20% chance
	default:
		return true // Normal and Darkness can be defended with anything
	}
}

func (r *Room) ApplyAttackModifiers(damage int, element Element) (int, Element) {
	// Apply permanent attack modifiers
	for _, effect := range r.GameState.PermanentEffects {
		if modifier, ok := effect.(ConvertDamageAction); ok {
			if modifier.ToElement != "" {
				element = modifier.ToElement
			}
			if modifier.Multiplier > 0 {
				damage = int(float64(damage) * modifier.Multiplier)
			}
		}
	}
	return damage, element
}

func (r *Room) ProcessDefenseTriggers(triggers map[Trigger]Action, defender *Player, ctx *DefenseContext) {
	// Process defense-specific triggers
}

// Helper functions
func (r *Room) GetAllPlayersExcept(exclude *Player) []*Player {
	players := make([]*Player, 0)
	for _, p := range r.Players {
		if p != exclude {
			players = append(players, p)
		}
	}
	return players
}

func (r *Room) GetActivePlayers() []*Player {
	active := make([]*Player, 0)
	for _, p := range r.Players {
		if p.Life > 0 {
			active = append(active, p)
		}
	}
	return active
}

func (r *Room) SelectTarget(attacker *Player) *Player {
	// Check for fog curse
	if attacker.Curses[Fog] > 0 {
		// Attack random player
		active := r.GetActivePlayers()
		if len(active) > 0 {
			return active[rand.Intn(len(active))]
		}
	}

	// Normal target selection (would be handled by UI)
	// For now, return first other player
	for _, p := range r.Players {
		if p != attacker && p.Life > 0 {
			return p
		}
	}
	return nil
}

func (r *Room) GetRandomPlayerExcept(exclude1, exclude2 *Player) *Player {
	active := r.GetActivePlayers()
	candidates := make([]*Player, 0)
	for _, p := range active {
		if p != exclude1 && p != exclude2 {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) > 0 {
		return candidates[rand.Intn(len(candidates))]
	}
	return nil
}

func (r *Room) RemovePlayer(player *Player) {
	for i, p := range r.Players {
		if p == player {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			break
		}
	}

	// Remove from game state order
	for i, p := range r.GameState.PlayerOrder {
		if p == player {
			r.GameState.PlayerOrder = append(r.GameState.PlayerOrder[:i], r.GameState.PlayerOrder[i+1:]...)
			break
		}
	}
}

func (p *Player) RemoveCardFromHand(cardID string) {
	for i, card := range p.Hand {
		if card.ID == cardID {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			break
		}
	}
}

func (r *Room) RemoveEffectFromSlice(slice []Effect, effect Effect) []Effect {
	for i, e := range slice {
		if e == effect {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func main() {
	fmt.Println("Card Game Engine Initialized")

	// Example usage
	room := NewRoom("Test Room")

	// Add players
	room.AddPlayer("Player 1")
	room.AddPlayer("Player 2")
	room.AddPlayer("Player 3")

	// Start game
	err := room.StartGame()
	if err != nil {
		fmt.Printf("Error starting game: %v\n", err)
		return
	}

	fmt.Printf("Game started with %d players\n", len(room.Players))
	fmt.Printf("Active player: %s\n", room.GameState.ActivePlayer.Name)
}
