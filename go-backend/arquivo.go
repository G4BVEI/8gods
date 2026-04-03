package main

import (
    "fmt"
    "math/rand"
    "time"
)
var players = []Player{}
var currentOrderByPlayee = []
var nextId = 1
func main() {
	Seed()
	shuffle()
	fmt.Println(players)
}
func Seed(){
	newPlayer("banana")
	newPlayer("maçã")
	newPlayer("morango")
	newPlayer("laranja")
	newPlayer("uva")
}
type Player struct {
	Name string
	Life, Money, Mana int
}
func newPlayer(name string) Player {
	p := Player{
		Name: name,
		Life: 40,
		Money: 15,
		Mana: 10,
	}
	players = append(players, p)
	return p
}
func shuffle(){
	rand.Seed(time.Now().UnixNano()) // seed once
	rand.Shuffle(len(players), func(i, j int) {
    players[i], players[j] = players[j], players[i]
	})
}
