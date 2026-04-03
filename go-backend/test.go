package main
import "fmt"
func main(){
	myArray := [6]int{1,2,3,4,5,6}
	sliceA := myArray[2:5] // value will be {3,4,5}
	sliceB := myArray[:3] // value will be {1,2,3}
	sliceC := myArray[3:] // value will be {4,5,6}
	sliceD := myArray[:] // value will be {1,2,3,4,5,6}
	fmt.Println(myArray)
	fmt.Println(sliceA)
	fmt.Println(sliceB)
	fmt.Println(sliceC)
	fmt.Println(sliceD)
}
