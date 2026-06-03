package main

import "fmt"

//gpp:derive String
type Color int

const (
	Red Color = iota
	Green
	Blue
)

//gpp:derive String
type Status int

const (
	StatusActive Status = iota + 1
	StatusInactive
	StatusPending
)

func main() {
	fmt.Println("")

	// Test derive String on Color
	fmt.Printf("Red=%s Blue=%s Unknown=%s\n", Red, Blue, Color(99))

	// Test derive String on Status
	fmt.Printf("Active=%s Inactive=%s Pending=%s\n", StatusActive, StatusInactive, StatusPending)
	fmt.Printf("StatusUnknown=%s\n", Status(0))
}
