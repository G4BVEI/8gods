package main
import (
	"fmt"
    "math/rand"
    "time"
)
type AttackCard struct {
	Damage int
	Element string
	CanStack bool
	Outcomes []AttackEffect
}
type AttackEffect struct {
	Probabily float64
	Targets []Player
	Effects []Effect
}
type Effect struct {
	priority Int

}
type Player struct{
	id int
}

func main() {
	getRandomPlayer()
}

func getRandomPlayer(){
	rand.Seed(time.Now().UnixNano())
    min := 1
    max := 2
    fmt.Println(rand.Intn(max - min + 1) + min)
}
