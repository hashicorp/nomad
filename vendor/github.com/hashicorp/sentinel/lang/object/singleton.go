package object

// The objects below are singleton values. These should never be modified.
var (
	Null  = &nullObj{}
	True  = &BoolObj{Value: true}
	False = &BoolObj{Value: false}
)

// Bool returns the object for the boolean value v. This will be either
// the singleton object True or False.
func Bool(v bool) *BoolObj {
	if v {
		return True
	} else {
		return False
	}
}
