---- MODULE SystemCoverageWF ----
EXTENDS SystemScheduler

NodesValue == { "n1", "n2" }
JobsValue  == { "j1", "j2" }

AttrsValue == [
    n1 |-> [ dc |-> "us", cores |-> 2 ],
    n2 |-> [ dc |-> "eu", cores |-> 1 ]
]

CapacityValue == [ n1 |-> 2, n2 |-> 1 ]

DemandValue == [ j1 |-> 1, j2 |-> 1 ]

ConstraintFnValue(a) == a.dc = "us"

ScoreFnValue(j,a) == IF j = "j1" THEN a.cores ELSE a.cores - 1

====