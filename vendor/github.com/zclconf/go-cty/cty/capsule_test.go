package cty

import "reflect"

type capsuleTestType1Native struct {
	name string
}

type capsuleTestType2Native struct {
	name string
}

var capsuleTestType1 = Capsule(
	"capsule test type 1",
	reflect.TypeOf(capsuleTestType1Native{}),
)

var capsuleTestType2 = Capsule(
	"capsule test type 2",
	reflect.TypeOf(capsuleTestType2Native{}),
)
