------------------------- MODULE SystemScheduler -------------------------
EXTENDS Naturals, Sequences, FiniteSets, TLC

\* ---------- Constants (to be set in TLC) ----------
CONSTANT
  Nodes,        \* finite set of nodes
  Jobs,         \* finite set of job identifiers
  Attrs,        \* Attrs is a function Nodes -> attribute record (abstract)
  Capacity,     \* Capacity is a function Nodes -> Nat
  Demand,       \* Demand is a function Jobs -> Nat
  ConstraintFn, \* ConstraintFn(job, attr) -> BOOLEAN
  ScoreFn       \* ScoreFn(job, attr) -> Nat

\* ---------- Helper sets ----------
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
  /\ ConstraintFn[j][Attrs[n]]
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
  <> (\A j \in curentJobs: \A c \in eligibleNodes:
        Eligible(j,c) => allocs[j][c] = 1)

\* Choose the best eligible (job,client) pair.
\* To be deterministic for TLC, we break ties by lexicographic order (smallest job then smallest client).
BestPairExists ==
  \E j \in currentJobs, c \in eligibleNodes: Eligible(j,c)

BestPair(j,c) ==
  /\ Eligible(j,c)
  /\ \A jb \in currentJobs, cb \in eligibleNodes:
       Eligible(jb,cb) =>
          ( ScoreFn[j][Attrs[c]] > ScoreFn[jb][Attrs[cb]]
            \/ (ScoreFn[j][Attrs[c]] = ScoreFn[jb][Attrs[cb]]
                /\ (j < jb \/ (j = jb /\ c <= cb)))
          )

\* ---------- PlusCal algorithm (inside comment block) ----------
(*
--algorithm MultiJobScheduler
variables
  eligibleNodes = Nodes;
  currentJobs   = Jobs;
  allocs        = [j \in Jobs |-> [n \in Nodes |-> 0]];

define
  UsedCap(c) == \sum_{j \in currentJobs} (allocs[j][c] * Demand[j]);
  Eligible(j,c) == (c \in eligibleNodes) /\ (j \in currentJobs)
                    /\ ConstraintFn(j, Attrs[c])
                    /\ (allocs[j][c] = 0)
                    /\ (UsedCap(c) + Demand[j] <= Capacity[c]);

  Score(j,c) == ScoreFn(j, Attrs[c]);
end define;

begin
MainLoop:
  while TRUE do
    either \* Place allocation on best eligible (job, node) pair
      if (\E j \in currentJobs, c \in eligibleNodes: Eligible(j,c)) then
        \* choose lexicographically smallest best pair to be deterministic in TLC
        with jb \in currentJobs do
          with nb \in eligibleNodes do
            if BestPair(jb,nb) then
              allocs[jb][nb] := 1;
            end if;
          end with;
        end with;
      end if;

    or \* Client joins
      with n \in Nodes do
        if n \notin eligibleNodes then
          eligibleNodes := eligibleNodes \cup {n};
        end if;
      end with;

    or \* Client leaves (evict all allocs on that client)
      with n \in eligibleNodes do
        eligibleNodes := eligibleNodes \ {n};
        \* free all allocations on that node
        with j \in Jobs do
          allocs[j][n] := 0;
        end with;
      end with;

    or \* Job added
      with j \in Jobs do
        if j \notin currentJobs then
          currentJobs := currentJobs \cup {j};
        end if;
      end with;

    or \* Job removed (evict all allocations of that job)
      with j \in currentJobs do
        currentJobs := currentJobs \ {j};
        with n \in Nodes do
          allocs[j][n] := 0;
        end with;
      end with;
    end either;
  end while;
end algorithm;
*)

\* ---------- Helpful invariants to check with TLC ----------
Inv ==
  /\ AllocRange
  /\ CapacitySafety
  /\ \A j \in currentJobs, c \in eligibleNodes:
       allocs[j][n] = 1 => ConstraintFn[j][Attrs[n]]
  /\ \A j \in currentJobs, n \in Nodes: allocs[j][n] = 1 => n \in eligibleNodes

=============================================================================
