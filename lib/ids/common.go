package ids

func Head(id string) string {
	return id[0:4]
}

func Tail(id string) string {
	return id[len(id)-4:]
}
