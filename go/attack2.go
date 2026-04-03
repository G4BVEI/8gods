package main
import (
    "fmt"
    "math/rand"
    "time"
)
type AttackCard struct {
	StackBase bool // can be the first attack card to be played, eg a 2x damage card shouldnt do so
	StackTop bool // can be stacked on top of other cards, only cards with extra effects should do so
	Outcomes []Outcome // defines all the possibilities that can from that card
}
type Outcome struct {
	Probabily float //prob this effect does get passed
	Attack Attack // the attack itself(next struct)
	Triggers map[Trigger]Effect// a map of all the effects with its triggers so we can mantain this all more easily and make a priority list
}
type Attack struct {
	Damage int // very simple attack object internals
	Element Element
}
