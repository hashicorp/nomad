package winsvc

var chanGraceExit = make(chan int)

func Shutdown_Channel() <-chan int {
	return chanGraceExit
}
