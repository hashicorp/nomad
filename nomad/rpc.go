package nomad

type RPCType byte

const (
	rpcNomad RPCType = iota
	rpcRaft
	rpcMultiplex
	rpcTLS
)
