package registry

type Zone struct {
	Name string
}

func NewZone(name string) Zone {
	return Zone{Name: name}
}
