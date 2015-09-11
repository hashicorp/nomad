package api

// Constraint is used to serialize a job placement constraint.
type Constraint struct {
	Hard    bool
	LTarget string
	RTarget string
	Operand string
	Weight  int
}

// HardConstraint is used to create a new hard constraint.
func HardConstraint(left, operand, right string) *Constraint {
	return constraint(left, operand, right, true, 0)
}

// SoftConstraint is used to create a new soft constraint. It
// takes an additional weight parameter to allow balancing
// multiple soft constraints amongst eachother.
func SoftConstraint(left, operand, right string, weight int) *Constraint {
	return constraint(left, operand, right, false, weight)
}

// constraint generates a new job placement constraint.
func constraint(left, operand, right string, hard bool, weight int) *Constraint {
	return &Constraint{
		Hard:    hard,
		LTarget: left,
		RTarget: right,
		Operand: operand,
		Weight:  weight,
	}
}
