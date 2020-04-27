package structs

type ReqType string

const (
	CmdReq    ReqType = "cmdline"
	CPUReq    ReqType = "cpu"
	TraceReq  ReqType = "trace"
	LookupReq ReqType = "lookup"
)
