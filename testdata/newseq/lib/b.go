package lib

type B struct{}

func (b *B) M() int {
	return 1
}

func (b B) MV() int {
	return 2
}

func (b B) Ident(v float64) float64 {
	return v
}

func FuncFromLib(v float64) float64 {
	return v
}
