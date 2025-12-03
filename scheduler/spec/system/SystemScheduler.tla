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

\* Used capacity on node n
RECURSIVE Sum(_)
Sum(S) == IF S = {} THEN 0 ELSE
            LET x == CHOOSE y \in S: TRUE IN x + Sum(S \ { x })

UsedCap(n) == Sum({ allocs[j][n] * Demand[j] : j \in currentJobs })

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

\* Desired coverage (for system jobs): for every job, every eligible node should have alloc=1
SystemCoverage ==
  <> (\A j \in currentJobs: \A n \in eligibleNodes:
        Eligible(j,n) => allocs[j][n] = 1)

\* Choose the best eligible (job,node) pair.
\* To be deterministic for TLC, we break ties by choosing deterministically
\* based on the maximum score, then using CHOOSE for tie-breaking
BestPairExists ==
  \E j \in currentJobs, n \in eligibleNodes: Eligible(j,n)

\* Helper: find maximum score among all eligible pairs
MaxScore ==
  LET eligiblePairs == { <<j,n>> \in currentJobs \X eligibleNodes : Eligible(j,n) }
  IN IF eligiblePairs = {} THEN 0
     ELSE LET scores == { ScoreFn(p[1], Attrs[p[2]]) : p \in eligiblePairs }
          IN CHOOSE s \in scores : \A s2 \in scores : s >= s2

BestPair(j,n) ==
  LET maxScorePairs == { <<jx,nx>> \in currentJobs \X eligibleNodes :
                          Eligible(jx,nx) /\ ScoreFn(jx, Attrs[nx]) = MaxScore }
  IN /\ Eligible(j,n)
     /\ ScoreFn(j, Attrs[n]) = MaxScore
     /\ <<j,n>> = CHOOSE p \in maxScorePairs : TRUE

\* Actual scheduling algorithm model
vars == << eligibleNodes, currentJobs, allocs >>

\* Individual actions for fairness
ScheduleJob ==
  /\ \E j \in currentJobs, n \in eligibleNodes:
       /\ BestPair(j,n)
       /\ allocs' = [allocs EXCEPT ![j][n] = 1]
  /\ UNCHANGED <<eligibleNodes, currentJobs>>

AddNode ==
  /\ \E n \in Nodes:
       /\ n \notin eligibleNodes
       /\ eligibleNodes' = (eligibleNodes \cup {n})
  /\ UNCHANGED <<currentJobs, allocs>>

RemoveNode ==
  /\ \E n \in eligibleNodes:
       /\ eligibleNodes' = eligibleNodes \ {n}
       /\ allocs' = [j \in Jobs |-> [allocs[j] EXCEPT ![n] = 0]]
  /\ UNCHANGED currentJobs

AddJob ==
  /\ \E j \in Jobs:
       /\ j \notin currentJobs
       /\ currentJobs' = (currentJobs \cup {j})
  /\ UNCHANGED <<eligibleNodes, allocs>>

RemoveJob ==
  /\ \E j \in currentJobs:
       /\ currentJobs' = currentJobs \ {j}
       /\ allocs' = [allocs EXCEPT ![j] = [n \in Nodes |-> 0]]
  /\ UNCHANGED eligibleNodes

Next == ScheduleJob \/ AddNode \/ RemoveNode \/ AddJob \/ RemoveJob

\* Specification with weak fairness on scheduling
Spec == Init /\ [][Next]_vars /\ WF_vars(ScheduleJob)

\* ---------- Helpful invariants to check with TLC ----------
Inv ==
  /\ AllocRange
  /\ CapacitySafety
  /\ \A j \in currentJobs, n \in eligibleNodes:
       allocs[j][n] = 1 => ConstraintFn(Attrs[n])
  /\ \A j \in currentJobs, n \in Nodes: allocs[j][n] = 1 => n \in eligibleNodes

====