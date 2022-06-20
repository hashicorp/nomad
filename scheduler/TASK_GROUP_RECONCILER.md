# Overview

## Hypothesis

We're trying to manage slots

## Component Model

TaskGroupReconciler (Aggregate Root)
-> allocSlot (Subordinate Aggregate Root)
    -> *TaskGroup
    -> Candidates []*Allocation
    
## 
