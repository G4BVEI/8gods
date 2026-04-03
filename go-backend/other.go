package main

import (
    "fmt"
    "math/rand"
    "time"
)

type Player struct {
    Id    int
    Name  string
    Life  int
    Money int
    Mana  int
}

func main() {
    // Create slice of players
    players := []Player{
        {Id: 1, Name: "Alice", Life: 40, Money: 15, Mana: 10},
        {Id: 2, Name: "Bob", Life: 40, Money: 15, Mana: 10},
        {Id: 3, Name: "Charlie", Life: 40, Money: 15, Mana: 10},
    }
    fmt.Println("Before shuffle:", players)

    // Shuffle
    rand.Seed(time.Now().UnixNano()) // seed once
    rand.Shuffle(len(players), func(i, j int) {
        players[i], players[j] = players[j], players[i]
    })

    fmt.Println("After shuffle:", players)
}
