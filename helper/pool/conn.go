package pool

type RPCType byte

const (
	RpcNomad     RPCType = 0x01
	RpcRaft              = 0x02
	RpcMultiplex         = 0x03
	RpcTLS               = 0x04
)
