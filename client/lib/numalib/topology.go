package numalib

type System struct {
	Sockets []Socket
}

// package
type Socket struct {
	ID    byte
	Cores []Core
}

type Core struct {
	ID      uint16
	Threads []Thread
}

type Thread struct {
	// eh

}
