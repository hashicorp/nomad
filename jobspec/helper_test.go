package jobspec

// These functions are copied from helper/funcs.go
// added here to avoid jobspec depending on any other package

// intToPtr returns the pointer to an int
func intToPtr(i int) *int {
	return &i
}

// int8ToPtr returns the pointer to an int8
func int8ToPtr(i int8) *int8 {
	return &i
}

// int64ToPtr returns the pointer to an int
func int64ToPtr(i int64) *int64 {
	return &i
}

// Uint64ToPtr returns the pointer to an uint64
func uint64ToPtr(u uint64) *uint64 {
	return &u
}
