package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// MINIMAL STDLIB WEBSOCKET
// ============================================================================

type WSConn struct {
	conn net.Conn
	buf  *bufio.ReadWriter
	mu   sync.Mutex
}

func wsUpgrade(w http.ResponseWriter, r *http.Request) (*WSConn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing websocket key", 400)
		return nil, fmt.Errorf("missing key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", 500)
		return nil, fmt.Errorf("no hijack")
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	buf.WriteString(resp)
	if err := buf.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	return &WSConn{conn: conn, buf: buf}, nil
}

func (ws *WSConn) ReadMessage() ([]byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(ws.buf, hdr); err != nil {
		return nil, err
	}
	opcode := hdr[0] & 0x0F
	if opcode == 8 {
		return nil, io.EOF // close frame
	}
	masked := hdr[1]&0x80 != 0
	plen := int64(hdr[1] & 0x7F)
	if plen == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(ws.buf, ext); err != nil {
			return nil, err
		}
		plen = int64(binary.BigEndian.Uint16(ext))
	} else if plen == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(ws.buf, ext); err != nil {
			return nil, err
		}
		plen = int64(binary.BigEndian.Uint64(ext))
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(ws.buf, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, plen)
	if _, err := io.ReadFull(ws.buf, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, nil
}

func (ws *WSConn) WriteMessage(data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	l := len(data)
	var hdr []byte
	if l < 126 {
		hdr = []byte{0x81, byte(l)}
	} else if l < 65536 {
		hdr = make([]byte, 4)
		hdr[0] = 0x81
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:], uint16(l))
	} else {
		hdr = make([]byte, 10)
		hdr[0] = 0x81
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:], uint64(l))
	}
	if _, err := ws.buf.Write(hdr); err != nil {
		return err
	}
	if _, err := ws.buf.Write(data); err != nil {
		return err
	}
	return ws.buf.Flush()
}

func (ws *WSConn) Close() {
	ws.mu.Lock()
	ws.buf.Write([]byte{0x88, 0x00})
	ws.buf.Flush()
	ws.mu.Unlock()
	ws.conn.Close()
}

// ============================================================================
// GAME TYPES
// ============================================================================

type Element = string

const (
	Normal   Element = "normal"
	Darkness Element = "darkness"
	Water    Element = "water"
	Fire     Element = "fire"
	Rock     Element = "rock"
	Wood     Element = "wood"
	Light    Element = "light"
)

type Curse = string

const (
	CurseCold      Curse = "cold"
	CurseFever     Curse = "fever"
	CurseHell      Curse = "hell"
	CurseHeaven    Curse = "heaven"
	CurseFog       Curse = "fog"
	CurseFlash     Curse = "flash"
	CurseDream     Curse = "dream"
	CurseDarkCloud Curse = "dark_cloud"
)

type GamePhase = string

const (
	PhaseWaiting   GamePhase = "waiting"
	PhaseTurn      GamePhase = "turn"
	PhaseDefending GamePhase = "defending"
	PhaseGameOver  GamePhase = "gameover"
)

type CardKind = int

const (
	KindAttack     CardKind = 1 << iota
	KindDefense
	KindConsumable
	KindMiracle
	KindSpecial
)

// CardDef is the static definition loaded at startup.
type CardDef struct {
	ID         string
	Name       string
	Kind       CardKind
	Price      int
	Deck       string // "attack" | "secondary"
	// attack fields
	StackBase  bool
	StackTop   bool
	Damage     int
	Element    Element
	AoE        bool
	// defense fields
	Defense    int
	DefElement Element // element type of this defense card (not the attack it blocks)
	// special fields
	IsReflect  bool
	IsSwing    bool
	HealAmount int
	ManaCost   int
}

// CardInstance is a live card in a player's hand or the store.
type CardInstance struct {
	CardDef
	IID string // unique instance ID
}

type Player struct {
	Name    string
	Life    int
	Money   int
	Mana    int
	Hand    []CardInstance
	Curses  map[Curse]bool
	IsAlive bool
	client  *Client // nil if disconnected
}

type AttackState struct {
	Attacker string
	Target   string
	Damage   int
	Element  Element
	Cards    []string // display names used
}

type GameState struct {
	Phase         GamePhase
	Players       []*Player
	PlayerOrder   []string
	CurrentTurn   int
	HasPlayed     []string
	Round         int
	AttackDeck    []CardDef
	SecondaryDeck []CardDef
	Store         []CardInstance
	PendingAttack *AttackState
	Winner        string
	Log           []string
}

// ============================================================================
// HARDCODED CARDS
// ============================================================================

func cardDefs() []CardDef {
	return []CardDef{
		// ── ATTACK DECK ─────────────────────────────────────────────────────
		{ID: "martelo-de-prata", Name: "Martelo de Prata",
			Kind: KindAttack, Deck: "attack", Price: 2,
			StackBase: true, Damage: 1, Element: Normal},
		{ID: "espada-do-heroi", Name: "Espada do Herói",
			Kind: KindAttack, Deck: "attack", Price: 8,
			StackBase: true, Damage: 12, Element: Normal},
		{ID: "lanca-de-gelo", Name: "Lança de Gelo",
			Kind: KindAttack, Deck: "attack", Price: 4,
			StackBase: true, Damage: 1, Element: Water},
		{ID: "faca-luciferiana", Name: "Faca Luciferiana",
			Kind: KindAttack, Deck: "attack", Price: 5,
			StackBase: true, Damage: 3, Element: Fire},
		{ID: "adaga-de-ferro", Name: "Adaga de Ferro",
			Kind: KindAttack, Deck: "attack", Price: 6,
			StackBase: true, Damage: 7, Element: Normal},
		// "+" stack cards
		{ID: "bola-de-fogo", Name: "Bola de Fogo",
			Kind: KindAttack, Deck: "attack", Price: 3,
			StackTop: true, Damage: 3, Element: Fire},
		{ID: "pena-de-ouro", Name: "Pena de Ouro",
			Kind: KindAttack, Deck: "attack", Price: 4,
			StackTop: true, Damage: 7, Element: Normal},

		// ── SECONDARY DECK ──────────────────────────────────────────────────
		// DefElement = the element TYPE of the card (fire card blocks water, water card blocks fire, etc.)
		{ID: "botadao-de-couro", Name: "Botadão de Couro",
			Kind: KindDefense, Deck: "secondary", Price: 2,
			Defense: 2, DefElement: Normal},
		{ID: "mao-de-ferro", Name: "Mão de Ferro",
			Kind: KindDefense, Deck: "secondary", Price: 5,
			Defense: 6, DefElement: Normal},
		{ID: "capa-infernal", Name: "Capa Infernal",
			Kind: KindDefense, Deck: "secondary", Price: 4,
			Defense: 6, DefElement: Fire}, // fire-type → blocks water attacks
		{ID: "fresco-defensor", Name: "Fresco Defensor",
			Kind: KindDefense, Deck: "secondary", Price: 5,
			Defense: 10, DefElement: Water}, // water-type → blocks fire attacks
		{ID: "espelho", Name: "Espelho",
			Kind: KindDefense, Deck: "secondary", Price: 7,
			IsReflect: true},
		{ID: "sangue-humanoide", Name: "Sangue Humanoide",
			Kind: KindConsumable, Deck: "secondary", Price: 3,
			HealAmount: 5},
	}
}

func buildDecks() ([]CardDef, []CardDef) {
	copies := map[string]int{
		"martelo-de-prata": 5, "espada-do-heroi": 3, "lanca-de-gelo": 4,
		"faca-luciferiana": 4, "adaga-de-ferro": 4, "bola-de-fogo": 5,
		"pena-de-ouro": 4, "botadao-de-couro": 5, "mao-de-ferro": 5,
		"capa-infernal": 4, "fresco-defensor": 4, "espelho": 3,
		"sangue-humanoide": 5,
	}
	var atk, sec []CardDef
	for _, d := range cardDefs() {
		n := copies[d.ID]
		for i := 0; i < n; i++ {
			if d.Deck == "attack" {
				atk = append(atk, d)
			} else {
				sec = append(sec, d)
			}
		}
	}
	return atk, sec
}

// ============================================================================
// ELEMENT LOGIC
// ============================================================================

// canDefendWith returns true when a defense card of type `def` can block an
// attack of type `att`.
//
//	Normal/Darkness → any defense works
//	Water           → only Fire-type defense
//	Fire            → only Water-type defense
//	Rock            → only Wood-type defense
//	Wood            → only Rock-type defense
//	Light           → nothing works
func canDefendWith(att, def Element) bool {
	switch att {
	case Normal, Darkness:
		return true
	case Water:
		return def == Fire
	case Fire:
		return def == Water
	case Rock:
		return def == Wood
	case Wood:
		return def == Rock
	case Light:
		return false
	}
	return true
}

// stackElements resolves two stacked attack layers.
//
//	light + dark    → normal
//	light + X       → X
//	X + X           → X     (same element stacks cleanly)
//	X + Y (≠ light) → normal
func stackElements(a, b Element) Element {
	if (a == Light && b == Darkness) || (a == Darkness && b == Light) {
		return Normal
	}
	if a == Light {
		return b
	}
	if b == Light {
		return a
	}
	if a == b {
		return a
	}
	return Normal
}

// ============================================================================
// GAME LOGIC
// ============================================================================

var (
	globalGame *GameState
	gameMu     sync.Mutex
	iidCounter int
)

func newIID(id string) string {
	iidCounter++
	return fmt.Sprintf("%s#%d", id, iidCounter)
}

func newInst(d CardDef) CardInstance {
	return CardInstance{CardDef: d, IID: newIID(d.ID)}
}

func shuffle[T any](s []T) {
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

func newGame() *GameState {
	atk, sec := buildDecks()
	shuffle(atk)
	shuffle(sec)
	return &GameState{
		Phase:         PhaseWaiting,
		AttackDeck:    atk,
		SecondaryDeck: sec,
		Log:           []string{"Aguardando jogadores..."},
	}
}

func (g *GameState) log(msg string) {
	g.Log = append(g.Log, msg)
	if len(g.Log) > 60 {
		g.Log = g.Log[len(g.Log)-60:]
	}
}

func (g *GameState) drawAtk() *CardInstance {
	if len(g.AttackDeck) == 0 {
		return nil
	}
	c := newInst(g.AttackDeck[0])
	g.AttackDeck = g.AttackDeck[1:]
	return &c
}

func (g *GameState) drawSec() *CardInstance {
	if len(g.SecondaryDeck) == 0 {
		return nil
	}
	c := newInst(g.SecondaryDeck[0])
	g.SecondaryDeck = g.SecondaryDeck[1:]
	return &c
}

func (g *GameState) player(name string) *Player {
	for _, p := range g.Players {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func (g *GameState) alive() []*Player {
	var out []*Player
	for _, p := range g.Players {
		if p.IsAlive {
			out = append(out, p)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (g *GameState) cardInHand(p *Player, iid string) *CardInstance {
	for i := range p.Hand {
		if p.Hand[i].IID == iid {
			return &p.Hand[i]
		}
	}
	return nil
}

func (g *GameState) removeFromHand(p *Player, iid string) bool {
	for i, c := range p.Hand {
		if c.IID == iid {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return true
		}
	}
	return false
}

// replenish draws one replacement card after a player uses one.
func (g *GameState) replenish(p *Player, deckType string) {
	if deckType == "attack" {
		if c := g.drawAtk(); c != nil {
			p.Hand = append(p.Hand, *c)
		}
	} else {
		if c := g.drawSec(); c != nil {
			p.Hand = append(p.Hand, *c)
		}
	}
}

func (g *GameState) startGame() {
	names := make([]string, len(g.Players))
	for i, p := range g.Players {
		p.Life, p.Mana, p.Money = 40, 10, 15
		p.Hand = nil
		p.Curses = map[Curse]bool{}
		p.IsAlive = true
		names[i] = p.Name
		for j := 0; j < 4; j++ {
			if c := g.drawAtk(); c != nil {
				p.Hand = append(p.Hand, *c)
			}
			if c := g.drawSec(); c != nil {
				p.Hand = append(p.Hand, *c)
			}
		}
	}
	shuffle(names)
	g.PlayerOrder = names
	g.CurrentTurn = 0
	g.HasPlayed = nil
	g.Round = 1
	g.Phase = PhaseTurn
	g.refreshStore()
	g.log(fmt.Sprintf("🎮 Jogo iniciado! Ordem: %s", strings.Join(names, " → ")))
	g.log(fmt.Sprintf("➡️  Vez de %s", g.activeName()))
}

func (g *GameState) refreshStore() {
	g.Store = nil
	for i := 0; i < 2; i++ {
		if c := g.drawSec(); c != nil {
			g.Store = append(g.Store, *c)
		}
	}
}

func (g *GameState) activeName() string {
	if len(g.PlayerOrder) == 0 {
		return ""
	}
	return g.PlayerOrder[g.CurrentTurn]
}

// ── Attack ──────────────────────────────────────────────────────────────────

func (g *GameState) doAttack(attackerName, targetName string, cardIIDs []string) error {
	attacker := g.player(attackerName)
	target := g.player(targetName)
	if attacker == nil || !attacker.IsAlive {
		return fmt.Errorf("atacante inválido")
	}
	if target == nil || !target.IsAlive {
		return fmt.Errorf("alvo inválido")
	}
	if len(cardIIDs) == 0 {
		return fmt.Errorf("selecione pelo menos uma carta")
	}
	base := g.cardInHand(attacker, cardIIDs[0])
	if base == nil {
		return fmt.Errorf("carta base não encontrada na mão")
	}
	if !base.StackBase {
		return fmt.Errorf("primeira carta precisa ser uma carta base de ataque")
	}

	totalDmg := base.Damage
	elem := base.Element
	names := []string{base.Name}

	for _, iid := range cardIIDs[1:] {
		card := g.cardInHand(attacker, iid)
		if card == nil {
			return fmt.Errorf("carta %s não está na mão", iid)
		}
		if !card.StackTop {
			return fmt.Errorf("'%s' não pode ser empilhada (precisa de '+')", card.Name)
		}
		totalDmg += card.Damage
		elem = stackElements(elem, card.Element)
		names = append(names, "+"+card.Name)
	}

	for _, iid := range cardIIDs {
		g.removeFromHand(attacker, iid)
	}
	g.replenish(attacker, "attack")

	g.PendingAttack = &AttackState{
		Attacker: attackerName,
		Target:   targetName,
		Damage:   totalDmg,
		Element:  elem,
		Cards:    names,
	}
	g.Phase = PhaseDefending
	g.log(fmt.Sprintf("⚔️  %s ataca %s: %s → %d de dano %s",
		attackerName, targetName, strings.Join(names, " "), totalDmg, elem))
	g.log(fmt.Sprintf("🛡️  %s, é hora de defender!", targetName))
	return nil
}

// ── Consumable ──────────────────────────────────────────────────────────────

func (g *GameState) doConsumable(playerName, iid string) error {
	p := g.player(playerName)
	if p == nil {
		return fmt.Errorf("jogador não encontrado")
	}
	card := g.cardInHand(p, iid)
	if card == nil {
		return fmt.Errorf("carta não está na mão")
	}
	if card.Kind&KindConsumable == 0 {
		return fmt.Errorf("não é um consumível")
	}
	g.removeFromHand(p, iid)
	if card.HealAmount > 0 {
		p.Life += card.HealAmount
		g.log(fmt.Sprintf("💊 %s usa %s → +%d HP (total: %d)", playerName, card.Name, card.HealAmount, p.Life))
	}
	g.replenish(p, "secondary")
	g.endTurn(playerName)
	return nil
}

// ── Defense ─────────────────────────────────────────────────────────────────

func (g *GameState) doDefend(defenderName string, cardIIDs []string) error {
	if g.PendingAttack == nil || g.PendingAttack.Target != defenderName {
		return fmt.Errorf("não é sua vez de defender")
	}
	defender := g.player(defenderName)
	attacker := g.player(g.PendingAttack.Attacker)
	atk := g.PendingAttack

	totalDef := 0
	for _, iid := range cardIIDs {
		card := g.cardInHand(defender, iid)
		if card == nil {
			return fmt.Errorf("carta %s não está na mão", iid)
		}
		// Reflect card: full damage goes back to attacker
		if card.IsReflect {
			g.removeFromHand(defender, iid)
			g.log(fmt.Sprintf("🪞 %s reflete! %d de dano %s vai para %s!", defenderName, atk.Damage, atk.Element, atk.Attacker))
			g.applyDamage(attacker, atk.Damage, atk.Element)
			g.PendingAttack = nil
			g.replenish(defender, "secondary")
			if g.Phase != PhaseGameOver {
				g.endTurn(atk.Attacker)
			}
			return nil
		}
		// Check elemental compatibility
		if !canDefendWith(atk.Element, card.DefElement) {
			return fmt.Errorf("'%s' não pode defender contra %s (elemento errado)", card.Name, atk.Element)
		}
		totalDef += card.Defense
		g.removeFromHand(defender, iid)
		g.replenish(defender, "secondary")
	}

	finalDmg := atk.Damage - totalDef
	if finalDmg < 0 {
		finalDmg = 0
	}

	// Darkness instakill: any unblocked darkness damage = instant death
	if atk.Element == Darkness && finalDmg > 0 {
		g.log(fmt.Sprintf("☠️  Dano de trevas não foi 100%% bloqueado — %s morre instantaneamente!", defenderName))
		defender.IsAlive = false
		g.checkWin()
		g.PendingAttack = nil
		if g.Phase != PhaseGameOver {
			g.endTurn(atk.Attacker)
		}
		return nil
	}

	if len(cardIIDs) == 0 {
		g.log(fmt.Sprintf("😬 %s não defendeu → toma %d de dano %s!", defenderName, finalDmg, atk.Element))
	} else {
		g.log(fmt.Sprintf("🛡️  %s bloqueou %d, toma %d de dano!", defenderName, totalDef, finalDmg))
	}
	if finalDmg > 0 {
		g.applyDamage(defender, finalDmg, atk.Element)
	}
	g.PendingAttack = nil
	if g.Phase != PhaseGameOver {
		g.endTurn(atk.Attacker)
	}
	return nil
}

// doSwing: defender redirects attack to any random player (including themselves).
func (g *GameState) doSwing(defenderName string) error {
	if g.PendingAttack == nil || g.PendingAttack.Target != defenderName {
		return fmt.Errorf("não é sua vez de defender")
	}
	atk := g.PendingAttack
	alive := g.alive()
	newTarget := alive[rand.Intn(len(alive))] // any player, including the defender
	g.log(fmt.Sprintf("🎲 %s rebate! Ataque redirecionado para %s!", defenderName, newTarget.Name))
	g.applyDamage(newTarget, atk.Damage, atk.Element)
	g.PendingAttack = nil
	if g.Phase != PhaseGameOver {
		g.endTurn(atk.Attacker)
	}
	return nil
}

func (g *GameState) applyDamage(p *Player, dmg int, elem Element) {
	if p == nil || !p.IsAlive {
		return
	}
	p.Life -= dmg
	if p.Life <= 0 {
		p.IsAlive = false
		g.log(fmt.Sprintf("💀 %s morreu!", p.Name))
		g.checkWin()
	}
}

func (g *GameState) checkWin() {
	a := g.alive()
	if len(a) == 1 {
		g.Winner = a[0].Name
		g.Phase = PhaseGameOver
		g.log(fmt.Sprintf("🏆 %s venceu!", g.Winner))
	} else if len(a) == 0 {
		g.Phase = PhaseGameOver
		g.log("Empate! Todos morreram.")
	}
}

// ── Buy ─────────────────────────────────────────────────────────────────────

func (g *GameState) doBuy(playerName, iid string) error {
	p := g.player(playerName)
	if p == nil {
		return fmt.Errorf("jogador não encontrado")
	}
	var card *CardInstance
	var idx int
	for i := range g.Store {
		if g.Store[i].IID == iid {
			card = &g.Store[i]
			idx = i
			break
		}
	}
	if card == nil {
		return fmt.Errorf("carta não encontrada na loja")
	}
	if p.Money < card.Price {
		return fmt.Errorf("dinheiro insuficiente (precisa %d, tem %d)", card.Price, p.Money)
	}
	p.Money -= card.Price
	p.Hand = append(p.Hand, *card)
	g.log(fmt.Sprintf("🛒 %s comprou '%s' por 💰%d", playerName, card.Name, card.Price))
	// Replace with a new card
	if c := g.drawSec(); c != nil {
		g.Store[idx] = *c
	} else {
		g.Store = append(g.Store[:idx], g.Store[idx+1:]...)
	}
	return nil
}

// ── Turn management ─────────────────────────────────────────────────────────

func (g *GameState) endTurn(playerName string) {
	if !contains(g.HasPlayed, playerName) {
		g.HasPlayed = append(g.HasPlayed, playerName)
	}
	// Count alive players who haven't played yet
	notPlayed := 0
	for _, p := range g.alive() {
		if !contains(g.HasPlayed, p.Name) {
			notPlayed++
		}
	}
	if notPlayed == 0 {
		g.endRound()
		return
	}
	// Advance to next player who is alive and hasn't played
	for i := 0; i < len(g.PlayerOrder); i++ {
		g.CurrentTurn = (g.CurrentTurn + 1) % len(g.PlayerOrder)
		next := g.player(g.PlayerOrder[g.CurrentTurn])
		if next != nil && next.IsAlive && !contains(g.HasPlayed, next.Name) {
			g.Phase = PhaseTurn
			g.log(fmt.Sprintf("➡️  Vez de %s", next.Name))
			return
		}
	}
	g.endRound()
}

func (g *GameState) endRound() {
	g.Round++
	g.HasPlayed = nil
	// Rebuild order with only alive players, preserving relative order
	var newOrder []string
	for _, name := range g.PlayerOrder {
		p := g.player(name)
		if p != nil && p.IsAlive {
			newOrder = append(newOrder, name)
		}
	}
	g.PlayerOrder = newOrder
	if len(newOrder) == 0 {
		return
	}
	g.CurrentTurn = 0
	g.Phase = PhaseTurn
	g.processDiseases()
	if g.Phase != PhaseGameOver {
		g.log(fmt.Sprintf("🔄 Rodada %d começa! Vez de %s", g.Round, g.activeName()))
	}
}

func (g *GameState) processDiseases() {
	for _, p := range g.alive() {
		for curse := range p.Curses {
			switch curse {
			case CurseCold:
				p.Life -= 1
				g.log(fmt.Sprintf("🤧 %s perde 1 HP (resfriado)", p.Name))
			case CurseFever:
				p.Life -= 2
				g.log(fmt.Sprintf("🤒 %s perde 2 HP (febre)", p.Name))
			case CurseHell:
				p.Life -= 5
				g.log(fmt.Sprintf("🔥 %s perde 5 HP (inferno)", p.Name))
			case CurseHeaven:
				p.Life += 5
				g.log(fmt.Sprintf("✨ %s ganha 5 HP (céu)", p.Name))
			}
			// 5% chance to worsen
			if rand.Intn(100) < 5 {
				chain := map[Curse]Curse{CurseCold: CurseFever, CurseFever: CurseHell, CurseHell: CurseHeaven}
				if next, ok := chain[curse]; ok {
					delete(p.Curses, curse)
					p.Curses[next] = true
					g.log(fmt.Sprintf("😱 Doença de %s piorou para %s!", p.Name, next))
				} else if curse == CurseHeaven {
					delete(p.Curses, curse)
					p.Life = 0
					g.log(fmt.Sprintf("⚰️  O céu de %s piorou — HP zerado!", p.Name))
				}
			}
		}
		if p.Life <= 0 && p.IsAlive {
			p.IsAlive = false
			g.log(fmt.Sprintf("💀 %s morreu pela doença!", p.Name))
			g.checkWin()
		}
	}
}

// ============================================================================
// WEBSOCKET HUB
// ============================================================================

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

var hub = &Hub{clients: map[*Client]bool{}}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// broadcast sends a personalized state snapshot to every connected client.
func (h *Hub) broadcast() {
	gameMu.Lock()
	g := globalGame
	gameMu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		msg := buildState(g, c.player)
		data, _ := json.Marshal(msg)
		_ = c.ws.WriteMessage(data)
	}
}

// ============================================================================
// CLIENT
// ============================================================================

type Client struct {
	ws     *WSConn
	player *Player
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := wsUpgrade(w, r)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	c := &Client{ws: ws}
	hub.add(c)
	defer func() {
		hub.remove(c)
		ws.Close()
		if c.player != nil {
			gameMu.Lock()
			c.player.client = nil
			// Remove from game if still waiting, otherwise mark dead
			if globalGame.Phase == PhaseWaiting {
				newPlayers := []*Player{}
				for _, p := range globalGame.Players {
					if p != c.player {
						newPlayers = append(newPlayers, p)
					}
				}
				globalGame.Players = newPlayers
				globalGame.log(fmt.Sprintf("❌ %s saiu da sala", c.player.Name))
			} else {
				c.player.IsAlive = false
				globalGame.log(fmt.Sprintf("❌ %s desconectou", c.player.Name))
				globalGame.checkWin()
				// If this was the active defender, auto-resolve
				if globalGame.Phase == PhaseDefending &&
					globalGame.PendingAttack != nil &&
					globalGame.PendingAttack.Target == c.player.Name {
					globalGame.PendingAttack = nil
					globalGame.Phase = PhaseTurn
				}
			}
			gameMu.Unlock()
			hub.broadcast()
		}
	}()

	// Send initial state to this client only
	gameMu.Lock()
	msg := buildState(globalGame, nil)
	gameMu.Unlock()
	data, _ := json.Marshal(msg)
	_ = ws.WriteMessage(data)

	// Read loop
	ws.conn.SetDeadline(time.Time{}) // no timeout for game connections
	for {
		raw, err := ws.ReadMessage()
		if err != nil {
			break
		}
		handleMsg(c, raw)
	}
}

// ============================================================================
// MESSAGE HANDLING
// ============================================================================

type InMsg struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	BaseCard string   `json:"base_card"`
	Stack    []string `json:"stack"`
	Target   string   `json:"target"`
	Cards    []string `json:"cards"`
	CardID   string   `json:"card_id"`
}

func sendErr(c *Client, msg string) {
	data, _ := json.Marshal(map[string]string{"type": "error", "message": msg})
	_ = c.ws.WriteMessage(data)
}

func handleMsg(c *Client, raw []byte) {
	var m InMsg
	if err := json.Unmarshal(raw, &m); err != nil {
		sendErr(c, "mensagem inválida")
		return
	}

	gameMu.Lock()
	g := globalGame
	var errStr string

	switch m.Type {

	case "join":
		if m.Name == "" {
			errStr = "nome obrigatório"
			break
		}
		if g.Phase != PhaseWaiting {
			errStr = "jogo em andamento"
			break
		}
		for _, p := range g.Players {
			if p.Name == m.Name {
				errStr = "nome já em uso"
				break
			}
		}
		if errStr != "" {
			break
		}
		if len(g.Players) >= 8 {
			errStr = "sala cheia"
			break
		}
		p := &Player{Name: m.Name, IsAlive: true, client: c, Curses: map[Curse]bool{}}
		c.player = p
		g.Players = append(g.Players, p)
		g.log(fmt.Sprintf("👋 %s entrou na sala!", m.Name))

	case "start":
		if g.Phase != PhaseWaiting {
			errStr = "jogo já iniciado"
			break
		}
		if len(g.Players) < 2 {
			errStr = "precisa de pelo menos 2 jogadores"
			break
		}
		g.startGame()

	case "reset":
		globalGame = newGame()
		// Re-register existing clients without players
		hub.mu.RLock()
		for cl := range hub.clients {
			cl.player = nil
		}
		hub.mu.RUnlock()
		globalGame.log("🔄 Sala resetada!")

	case "attack":
		if g.Phase != PhaseTurn {
			errStr = "não é fase de ataque"
			break
		}
		if c.player == nil || c.player.Name != g.activeName() {
			errStr = "não é sua vez"
			break
		}
		iids := []string{m.BaseCard}
		for _, s := range m.Stack {
			if s != "" {
				iids = append(iids, s)
			}
		}
		if err := g.doAttack(c.player.Name, m.Target, iids); err != nil {
			errStr = err.Error()
		}

	case "consumable":
		if g.Phase != PhaseTurn {
			errStr = "não é sua vez"
			break
		}
		if c.player == nil || c.player.Name != g.activeName() {
			errStr = "não é sua vez"
			break
		}
		if err := g.doConsumable(c.player.Name, m.CardID); err != nil {
			errStr = err.Error()
		}

	case "defend":
		if g.Phase != PhaseDefending {
			errStr = "não é fase de defesa"
			break
		}
		if g.PendingAttack == nil || c.player == nil || c.player.Name != g.PendingAttack.Target {
			errStr = "não é você que está sendo atacado"
			break
		}
		if err := g.doDefend(c.player.Name, m.Cards); err != nil {
			errStr = err.Error()
		}

	case "pass":
		if g.Phase != PhaseDefending {
			errStr = "não é fase de defesa"
			break
		}
		if g.PendingAttack == nil || c.player == nil || c.player.Name != g.PendingAttack.Target {
			errStr = "não é você que está sendo atacado"
			break
		}
		if err := g.doDefend(c.player.Name, nil); err != nil {
			errStr = err.Error()
		}

	case "swing":
		if g.Phase != PhaseDefending {
			errStr = "não é fase de defesa"
			break
		}
		if g.PendingAttack == nil || c.player == nil || c.player.Name != g.PendingAttack.Target {
			errStr = "não é você que está sendo atacado"
			break
		}
		// Must have a swing card in hand (IsSwing flag — none in current deck, but extensible)
		hasSwing := false
		for _, card := range c.player.Hand {
			if card.IsSwing {
				hasSwing = true
				break
			}
		}
		if !hasSwing {
			errStr = "nenhuma carta de rebate na mão"
			break
		}
		if err := g.doSwing(c.player.Name); err != nil {
			errStr = err.Error()
		}

	case "buy":
		if c.player == nil {
			errStr = "entre no jogo primeiro"
			break
		}
		if err := g.doBuy(c.player.Name, m.CardID); err != nil {
			errStr = err.Error()
		}

	default:
		errStr = "tipo de mensagem desconhecido"
	}

	gameMu.Unlock()

	if errStr != "" {
		sendErr(c, errStr)
		return
	}
	hub.broadcast()
}

// ============================================================================
// STATE SERIALISATION
// ============================================================================

type StateMsg struct {
	Type            string       `json:"type"`
	Phase           GamePhase    `json:"phase"`
	Round           int          `json:"round"`
	ActivePlayer    string       `json:"active_player"`
	DefendingPlayer string       `json:"defending_player,omitempty"`
	PendingAttack   *AttackState `json:"pending_attack,omitempty"`
	Players         []PView      `json:"players"`
	You             *PView       `json:"you,omitempty"`
	Store           []CView      `json:"store"`
	Log             []string     `json:"log"`
	Winner          string       `json:"winner,omitempty"`
}

type PView struct {
	Name     string   `json:"name"`
	Life     int      `json:"life"`
	Mana     int      `json:"mana"`
	Money    int      `json:"money"`
	Alive    bool     `json:"alive"`
	Curses   []string `json:"curses"`
	Hand     []CView  `json:"hand,omitempty"`
	HandSize int      `json:"hand_size"`
}

type CView struct {
	IID        string  `json:"iid"`
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Desc       string  `json:"desc"`
	Price      int     `json:"price"`
	StackBase  bool    `json:"stack_base,omitempty"`
	StackTop   bool    `json:"stack_top,omitempty"`
	IsReflect  bool    `json:"is_reflect,omitempty"`
	IsSwing    bool    `json:"is_swing,omitempty"`
	IsConsume  bool    `json:"is_consume,omitempty"`
	IsDefense  bool    `json:"is_defense,omitempty"`
	Element    Element `json:"element,omitempty"`
	DefElement Element `json:"def_element,omitempty"`
	Damage     int     `json:"damage,omitempty"`
	Defense    int     `json:"defense,omitempty"`
}

func toView(c CardInstance) CView {
	desc := ""
	switch {
	case c.StackBase:
		desc = fmt.Sprintf("%d dano %s", c.Damage, c.Element)
	case c.StackTop:
		desc = fmt.Sprintf("+%d dano %s", c.Damage, c.Element)
	case c.IsReflect:
		desc = "Reflete tudo!"
	case c.Kind&KindDefense != 0 && c.Defense > 0:
		desc = fmt.Sprintf("Defende %d", c.Defense)
		if c.DefElement != "" && c.DefElement != Normal {
			desc += fmt.Sprintf(" (bloqueio %s)", c.DefElement)
		}
	case c.Kind&KindConsumable != 0 && c.HealAmount > 0:
		desc = fmt.Sprintf("Cura %d HP", c.HealAmount)
	}
	return CView{
		IID: c.IID, ID: c.ID, Name: c.Name, Desc: desc, Price: c.Price,
		StackBase: c.StackBase, StackTop: c.StackTop,
		IsReflect: c.IsReflect, IsSwing: c.IsSwing,
		IsConsume:  c.Kind&KindConsumable != 0,
		IsDefense:  c.Kind&KindDefense != 0 && !c.StackBase,
		Element:    c.Element, DefElement: c.DefElement,
		Damage: c.Damage, Defense: c.Defense,
	}
}

func buildState(g *GameState, me *Player) StateMsg {
	players := make([]PView, 0, len(g.Players))
	for _, p := range g.Players {
		curses := make([]string, 0)
		for c := range p.Curses {
			curses = append(curses, c)
		}
		pv := PView{
			Name: p.Name, Life: p.Life, Mana: p.Mana, Money: p.Money,
			Alive: p.IsAlive, Curses: curses, HandSize: len(p.Hand),
		}
		players = append(players, pv)
	}
	store := make([]CView, 0, len(g.Store))
	for _, c := range g.Store {
		store = append(store, toView(c))
	}
	msg := StateMsg{
		Type: "state", Phase: g.Phase, Round: g.Round,
		ActivePlayer: g.activeName(),
		Players:      players, Store: store, Log: g.Log, Winner: g.Winner,
	}
	if g.PendingAttack != nil {
		msg.DefendingPlayer = g.PendingAttack.Target
		msg.PendingAttack = g.PendingAttack
	}
	if me != nil {
		hand := make([]CView, 0, len(me.Hand))
		for _, c := range me.Hand {
			hand = append(hand, toView(c))
		}
		curses := make([]string, 0)
		for c := range me.Curses {
			curses = append(curses, c)
		}
		you := &PView{
			Name: me.Name, Life: me.Life, Mana: me.Mana, Money: me.Money,
			Alive: me.IsAlive, Curses: curses, Hand: hand, HandSize: len(me.Hand),
		}
		msg.You = you
	}
	return msg
}

// ============================================================================
// FRONTEND
// ============================================================================

const html = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<title>Card Game</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{background:#0c0c0c;color:#ddd;font-family:monospace;font-size:13px;height:100vh;display:flex;flex-direction:column}
/* ── JOIN ── */
#join{display:flex;flex-direction:column;align-items:center;justify-content:center;height:100vh;gap:12px}
#join h1{color:#ffd700;font-size:22px;letter-spacing:2px}
#join input{background:#111;border:1px solid #444;color:#fff;padding:8px 12px;font:inherit;width:200px;border-radius:3px}
/* ── GAME ── */
#game{display:none;flex-direction:column;height:100vh;overflow:hidden}
#topbar{background:#111;border-bottom:1px solid #2a2a2a;padding:6px 10px;display:flex;gap:10px;flex-wrap:wrap;align-items:center;min-height:52px}
.pcard{padding:3px 10px;border:1px solid #333;border-radius:4px;white-space:nowrap}
.pcard.me{border-color:#555}
.pcard.active{border-color:#ffd700}
.pcard.defending{border-color:#f55}
.pcard.dead{opacity:.35;text-decoration:line-through}
.pname{font-weight:bold;font-size:12px}
.pstats{color:#888;font-size:11px}
#mid{display:flex;flex:1;overflow:hidden}
#log-wrap{flex:1;overflow-y:auto;padding:8px 10px;background:#0a0a0a;line-height:1.7}
#log-wrap div{border-bottom:1px solid #141414;padding:1px 0}
#right{width:310px;border-left:1px solid #222;display:flex;flex-direction:column;overflow:hidden}
#store-pane{padding:8px;border-bottom:1px solid #222}
#store-pane h3{color:#ffd700;font-size:11px;margin-bottom:5px;text-transform:uppercase;letter-spacing:1px}
#hand-pane{flex:1;overflow-y:auto;padding:8px}
#hand-pane h3{color:#88ff88;font-size:11px;margin-bottom:5px;text-transform:uppercase;letter-spacing:1px}
#action-pane{padding:8px;border-top:1px solid #222;background:#0e0e0e;min-height:110px}
#action-pane h3{color:#f88;font-size:11px;margin-bottom:6px;text-transform:uppercase;letter-spacing:1px}
.hint{color:#555;font-size:11px;margin-bottom:6px}
.scard,.hcard{background:#141414;border:1px solid #2a2a2a;border-radius:3px;padding:5px 8px;margin-bottom:4px;cursor:pointer}
.scard:hover{border-color:#ffd700}
.hcard:hover{border-color:#88ff88}
.hcard.sel{border-color:#ffd700;background:#181800}
.cname{font-size:12px;font-weight:bold}
.cdesc{color:#666;font-size:11px;margin-top:1px}
.ctag{font-size:10px;padding:1px 4px;border-radius:2px;margin-right:3px;margin-top:2px;display:inline-block}
.t-atk{background:#200;color:#f88;border:1px solid #f44}
.t-def{background:#020;color:#8f8;border:1px solid #4f4}
.t-plus{background:#210;color:#fa4;border:1px solid #f80}
.t-con{background:#002;color:#88f;border:1px solid #44f}
.t-ref{background:#112;color:#8cf;border:1px solid #4af}
.targets{display:flex;flex-wrap:wrap;gap:4px;margin-bottom:6px}
btn,button{background:#181818;border:1px solid #444;color:#ccc;padding:5px 10px;cursor:pointer;font:inherit;border-radius:3px;margin-right:3px;margin-bottom:3px}
button:hover{background:#222;border-color:#888}
button.y{border-color:#ffd700;color:#ffd700}
button.y:hover{background:#181800}
button.g{border-color:#4f4;color:#8f8}
button.g:hover{background:#020}
button.r{border-color:#f44;color:#f88}
button.r:hover{background:#200}
#sbar{background:#111;border-top:1px solid #1a1a1a;padding:3px 10px;font-size:11px;color:#555}
</style>
</head>
<body>

<div id="join">
  <h1>🃏 CARD GAME</h1>
  <input id="ni" type="text" placeholder="Seu nome..." maxlength="20"/>
  <button class="y" onclick="join()">Entrar na Sala</button>
</div>

<div id="game">
  <div id="topbar"></div>
  <div id="mid">
    <div id="log-wrap"></div>
    <div id="right">
      <div id="store-pane"><h3>🏪 Loja</h3><div id="store-list"></div></div>
      <div id="hand-pane"><h3>🃏 Sua Mão</h3><div id="hand-list"></div></div>
      <div id="action-pane"><h3>⚡ Ação</h3><div id="action"></div></div>
    </div>
  </div>
  <div id="sbar">Conectando...</div>
</div>

<script>
let ws,me='',st=null,sel=[],logLen=0;

function join(){
  me=document.getElementById('ni').value.trim();
  if(!me)return;
  ws=new WebSocket('ws://'+location.host+'/ws');
  ws.onopen=()=>{
    send({type:'join',name:me});
    document.getElementById('join').style.display='none';
    document.getElementById('game').style.display='flex';
  };
  ws.onmessage=e=>{
    const m=JSON.parse(e.data);
    if(m.type==='error'){showErr(m.message);return;}
    if(m.type==='state'){st=m;render();}
  };
  ws.onclose=()=>{document.getElementById('sbar').textContent='Desconectado. Recarregue.';}
}
function send(o){ws&&ws.readyState===1&&ws.send(JSON.stringify(o));}
function showErr(msg){const b=document.getElementById('sbar');b.style.color='#f88';b.textContent='⚠ '+msg;setTimeout(()=>{b.style.color='';render();},2500);}

function render(){
  if(!st)return;
  renderTop();renderLog();renderStore();renderHand();renderAction();
  const labels={waiting:'Aguardando',turn:'Em jogo',defending:'Defendendo',gameover:'Fim de jogo'};
  document.getElementById('sbar').textContent=
    'Rodada '+st.round+' | '+labels[st.phase]+
    (st.active_player?' | Vez de: '+st.active_player:'');
}

function renderTop(){
  const bar=document.getElementById('topbar');
  bar.innerHTML='';
  if(st.phase==='waiting'){
    const span=document.createElement('span');
    span.style.color='#888';
    span.textContent='Sala de espera — '+(st.players||[]).length+' jogador(es)';
    bar.appendChild(span);
    if((st.players||[]).length>=2){
      const b=document.createElement('button');
      b.textContent='▶ Iniciar';b.className='y';
      b.onclick=()=>send({type:'start'});
      bar.appendChild(b);
    }
    for(const p of(st.players||[])){
      const d=document.createElement('div');
      d.className='pcard';
      d.innerHTML='<div class="pname">'+p.name+(p.name===me?' (você)':'')+'</div>';
      bar.appendChild(d);
    }
    return;
  }
  if(st.phase==='gameover'){
    const b=document.createElement('button');
    b.textContent='🔄 Nova Sala';b.className='y';
    b.onclick=()=>send({type:'reset'});
    bar.appendChild(b);
  }
  for(const p of(st.players||[])){
    const d=document.createElement('div');
    let cls='pcard';
    if(!p.alive)cls+=' dead';
    else if(p.name===st.active_player&&st.phase==='turn')cls+=' active';
    else if(p.name===st.defending_player)cls+=' defending';
    if(p.name===me)cls+=' me';
    d.className=cls;
    const curses=p.curses&&p.curses.length?' ['+p.curses.join(',')+']':'';
    d.innerHTML='<div class="pname">'+p.name+(p.name===me?' (você)':'')+curses+'</div>'+
      '<div class="pstats">❤'+p.life+' 💧'+p.mana+' 💰'+p.money+' 🃏'+p.hand_size+'</div>';
    bar.appendChild(d);
  }
}

function renderLog(){
  const el=document.getElementById('log-wrap');
  if(!st.log||st.log.length===logLen)return;
  el.innerHTML='';
  for(const l of st.log){const d=document.createElement('div');d.textContent=l;el.appendChild(d);}
  el.scrollTop=el.scrollHeight;
  logLen=st.log.length;
}

function renderStore(){
  const el=document.getElementById('store-list');
  el.innerHTML='';
  for(const c of(st.store||[])){
    const d=document.createElement('div');d.className='scard';
    d.innerHTML='<div style="display:flex;justify-content:space-between"><span class="cname">'+c.name+'</span><span style="color:#ffd700">💰'+c.price+'</span></div>'+
      '<div class="cdesc">'+c.desc+'</div>';
    d.onclick=()=>{if(confirm('Comprar '+c.name+' por 💰'+c.price+'?'))send({type:'buy',card_id:c.iid});};
    el.appendChild(d);
  }
}

function renderHand(){
  const el=document.getElementById('hand-list');
  el.innerHTML='';
  if(!st.you||!st.you.hand)return;
  for(const c of st.you.hand){
    const d=document.createElement('div');
    d.className='hcard'+(sel.includes(c.iid)?' sel':'');
    let tags='';
    if(c.stack_base)tags+='<span class="ctag t-atk">ATAQUE</span>';
    if(c.stack_top) tags+='<span class="ctag t-plus">+STACK</span>';
    if(c.is_reflect)tags+='<span class="ctag t-ref">REFLEXO</span>';
    if(c.is_defense&&!c.stack_base)tags+='<span class="ctag t-def">DEFESA</span>';
    if(c.is_consume)tags+='<span class="ctag t-con">CONSUMÍVEL</span>';
    d.innerHTML='<div class="cname">'+c.name+'</div><div class="cdesc">'+c.desc+'</div><div>'+tags+'</div>';
    d.onclick=()=>toggle(c.iid);
    el.appendChild(d);
  }
}

function toggle(iid){
  const i=sel.indexOf(iid);
  if(i>=0)sel.splice(i,1);else sel.push(iid);
  renderHand();renderAction();
}

function renderAction(){
  const el=document.getElementById('action');
  el.innerHTML='';
  if(!st)return;

  if(st.phase==='waiting'){el.innerHTML='<div class="hint">Aguardando jogadores...</div>';return;}
  if(st.phase==='gameover'){
    el.innerHTML='<div style="color:#ffd700;font-size:15px">🏆 '+(st.winner?st.winner+' venceu!':'Fim de jogo')+'</div>';
    return;
  }

  const myTurn=st.active_player===me&&st.phase==='turn';
  const defending=st.defending_player===me&&st.phase==='defending';

  if(myTurn){
    const hand=(st.you&&st.you.hand)||[];
    const selected=hand.filter(c=>sel.includes(c.iid));
    const base=selected.find(c=>c.stack_base);
    const consumable=selected.find(c=>c.is_consume&&!c.stack_base&&!c.stack_top);

    if(selected.length===0){
      el.innerHTML='<div class="hint">É sua vez! Selecione uma carta na mão.</div>';
    }
    if(consumable&&selected.length===1){
      const b=document.createElement('button');
      b.textContent='💊 Usar '+consumable.name;b.className='g';
      b.onclick=()=>{send({type:'consumable',card_id:consumable.iid});sel=[];};
      el.appendChild(b);
    }
    if(base){
      const stack=selected.filter(c=>c.stack_top);
      const info=document.createElement('div');
      info.className='hint';
      const total=selected.reduce((s,c)=>s+(c.damage||0),0);
      info.textContent='Ataque: '+total+' dano. Escolha um alvo:';
      el.appendChild(info);
      const tgt=document.createElement('div');tgt.className='targets';
      for(const p of(st.players||[])){
        if(p.name!==me&&p.alive){
          const b=document.createElement('button');
          b.textContent='⚔️ '+p.name;b.className='r';
          b.onclick=(()=>{
            const pname=p.name;
            return()=>{
              send({type:'attack',base_card:base.iid,stack:stack.map(c=>c.iid),target:pname});
              sel=[];
            };
          })();
          tgt.appendChild(b);
        }
      }
      el.appendChild(tgt);
    }
    const buyHint=document.createElement('div');
    buyHint.className='hint';buyHint.style.marginTop='6px';
    buyHint.textContent='💰 Clique em uma carta da loja para comprar (qualquer hora).';
    el.appendChild(buyHint);

  } else if(defending){
    const atk=st.pending_attack;
    const info=document.createElement('div');
    info.style.color='#f88';info.style.marginBottom='6px';
    info.innerHTML='⚔️ <b>'+atk.attacker+'</b> → '+atk.damage+' dano <b>'+atk.element+'</b>';
    el.appendChild(info);

    const hand=(st.you&&st.you.hand)||[];
    const selected=hand.filter(c=>sel.includes(c.iid));
    const hasReflect=selected.some(c=>c.is_reflect);
    const defTotal=selected.filter(c=>c.is_defense).reduce((s,c)=>s+(c.defense||0),0);

    if(selected.length>0){
      const info2=document.createElement('div');
      info2.className='hint';
      info2.textContent=hasReflect?'Reflexo selecionado!':'Bloqueando: '+defTotal+' de dano';
      el.appendChild(info2);
    }

    const db=document.createElement('button');
    db.className='g';
    db.textContent=hasReflect?'🪞 Refletir':'🛡️ Defender'+(selected.length?' ('+selected.length+' carta'+(selected.length>1?'s':'')+')':', sem cartas');
    db.onclick=()=>{
      if(hasReflect){const r=selected.find(c=>c.is_reflect);send({type:'defend',cards:[r.iid]});}
      else send({type:'defend',cards:selected.map(c=>c.iid)});
      sel=[];
    };
    el.appendChild(db);

    const pb=document.createElement('button');
    pb.className='r';pb.textContent='😬 Tomar dano';
    pb.onclick=()=>{send({type:'pass'});sel=[];};
    el.appendChild(pb);

    const hint=document.createElement('div');
    hint.className='hint';hint.style.marginTop='4px';
    hint.textContent='Selecione cartas de defesa na mão antes de clicar Defender.';
    el.appendChild(hint);

  } else {
    const msg=st.phase==='defending'
      ?'⏳ Aguardando '+st.defending_player+' defender...'
      :'⏳ Vez de '+(st.active_player||'...');
    el.innerHTML='<div class="hint">'+msg+'</div>';
  }
}

document.getElementById('ni').addEventListener('keydown',e=>{if(e.key==='Enter')join();});
</script>
</body>
</html>`

// ============================================================================
// HTTP + MAIN
// ============================================================================

func main() {
	rand.Seed(time.Now().UnixNano())
	globalGame = newGame()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	http.HandleFunc("/ws", handleWS)

	addr := ":8080"
	log.Printf("🃏  Card Game → http://localhost%s", addr)
	log.Printf("    Abra várias abas para jogar com múltiplas instâncias")
	log.Fatal(http.ListenAndServe(addr, nil))
}
