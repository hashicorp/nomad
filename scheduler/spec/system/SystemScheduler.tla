---- MODULE SystemScheduler ----
EXTENDS Naturals, Sequences, FiniteSets, TLC

CONSTANT
  Nodes,           \* finite set of nodes
  Jobs,            \* finite set of job identifiers
  Attrs,           \* Attrs is a function Nodes -> attribute record (abstract)
  Capacity,        \* Capacity is a function Nodes -> Nat
  Demand,          \* Demand is a function Jobs -> Nat
  ConstraintFn(_), \* ConstraintFn(attr) -> BOOLEAN
  ScoreFn(_,_)     \* ScoreFn(job, attr) -> Nat

NodeSet == Nodes
JobSet  == Jobs

VARIABLE
  eligibleNodes, \* subset of Nodes that are currently eligible
  allocs,        \* allocs[j][n] = 0/1 indicating allocation of job j on node n
  currentJobs    \* subset of Jobs that exist (allows job add/remove)

\* Initialize variables
Init ==
  /\ eligibleNodes = Nodes
  /\ currentJobs = Jobs
  /\ allocs = [j \in Jobs |-> [n \in Nodes |-> 0]]

\* Used capacity on client c
RECURSIVE Sum(_)
Sum(S) == IF S = {} THEN 0 ELSE
            LET x == CHOOSE y \in S: TRUE IN x + Sum(S \ { x })

UsedCap(c) == Sum({ allocs[j][c] * Demand[j] : j \in currentJobs })

\* Whether job j can run on node n (constraints + capacity + presence)
Eligible(j,n) ==
  /\ n \in eligibleNodes
  /\ j \in currentJobs
  /\ ConstraintFn(Attrs[n])
  /\ allocs[j][n] = 0
  /\ UsedCap(n) + Demand[j] <= Capacity[n]

\* Safety: no node exceeds capacity
CapacitySafety ==
  \A n \in Nodes: UsedCap(n) <= Capacity[n]

\* Safety: allocs are only 0 or 1
AllocRange ==
  \A j \in Jobs: \A n \in Nodes: allocs[j][n] \in {0, 1}

\* Desired coverage (for system jobs): for every job, every eligible client should have alloc=1
\* SystemCoverage ==
\*   \A j \in currentJobs: \A c \in eligibleNodes:
\*      (ConstraintFn[j][Attrs[c]] /\ (UsedCap(c) + Demand[j] <= Capacity[c]))
\*         => allocs[j][c] = 1

SystemCoverage ==
  <> (\A j \in currentJobs: \A c \in eligibleNodes:
        Eligible(j,c) => allocs[j][c] = 1)

\* Choose the best eligible (job,client) pair.
\* To be deterministic for TLC, we break ties by lexicographic order (smallest
\* job then smallest client).
BestPairExists ==
  \E j \in currentJobs, c \in eligibleNodes: Eligible(j,c)

BestPair(j,c) ==
  /\ Eligible(j,c)
  /\ \A jb \in currentJobs, cb \in eligibleNodes:
       Eligible(jb,cb) =>
          ( ScoreFn(j, Attrs[c]) > ScoreFn(jb, Attrs[cb])
            \/ (ScoreFn(j, Attrs[c]) = ScoreFn(jb, Attrs[cb])
                /\ (j < jb \/ (j = jb /\ c <= cb)))
          )

\* Actual scheduling algorithm model
vars == << eligibleNodes, currentJobs, allocs >>

Next == \/ /\ IF (\E j \in currentJobs, c \in eligibleNodes: Eligible(j,c))
                 THEN /\ \E jb \in currentJobs:
                           \E nb \in eligibleNodes:
                             IF BestPair(jb,nb)
                                THEN /\ allocs' = [allocs EXCEPT ![jb][nb] = 1]
                                ELSE /\ TRUE
                                     /\ UNCHANGED allocs
                 ELSE /\ TRUE
                      /\ UNCHANGED allocs
           /\ UNCHANGED <<eligibleNodes, currentJobs>>
        \/ /\ \E n \in Nodes:
                IF n \notin eligibleNodes
                   THEN /\ eligibleNodes' = (eligibleNodes \cup {n})
                   ELSE /\ TRUE
                        /\ UNCHANGED eligibleNodes
           /\ UNCHANGED <<currentJobs, allocs>>
        \/ /\ \E n \in eligibleNodes:
                /\ eligibleNodes' = eligibleNodes \ {n}
                /\ \E j \in Jobs:
                     allocs' = [allocs EXCEPT ![j][n] = 0]
           /\ UNCHANGED currentJobs
        \/ /\ \E j \in Jobs:
                IF j \notin currentJobs
                   THEN /\ currentJobs' = (currentJobs \cup {j})
                   ELSE /\ TRUE
                        /\ UNCHANGED currentJobs
           /\ UNCHANGED <<eligibleNodes, allocs>>
        \/ /\ \E j \in currentJobs:
                /\ currentJobs' = currentJobs \ {j}
                /\ \E n \in Nodes:
                     allocs' = [allocs EXCEPT ![j][n] = 0]
           /\ UNCHANGED eligibleNodes

Spec == Init /\ [][Next]_vars

\* ---------- Helpful invariants to check with TLC ----------
Inv ==
  /\ AllocRange
  /\ CapacitySafety
  /\ \A j \in currentJobs, n \in eligibleNodes:
       allocs[j][n] = 1 => ConstraintFn(Attrs[n])
  /\ \A j \in currentJobs, n \in Nodes: allocs[j][n] = 1 => n \in eligibleNodes

====
