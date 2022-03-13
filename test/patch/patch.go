package main

var Data *int

var NewHiddenFunc func()

func NewFunc() {
	println("New Func: ", *Data)
	*Data += 15
	NewHiddenFunc()
}
