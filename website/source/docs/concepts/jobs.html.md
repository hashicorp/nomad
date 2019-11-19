---
layout: "docs"
page_title: "Concepts-Nomad Jobs"
sidebar_current: "docs-concepts-jobs"
description: |-
  Welcome to the Nomad documentation! This documentation is more of a reference
  guide for all available features and options of Nomad.
---

# Nomad Jobs

Nomad is a scheduler designed to run workload as efficiently as possible over
a fleet of client machines, called Nomad clients. Nomad calls these running
workloads "jobs".

## A working definition

Since "job" is a fairly generic word in terms of scheduling and computing, let's
start with a specific definition that we will be using throughout the
documentation.

> **job** _(noun)_ - 
>
> 1. a specification in HCL or JSON that defines the structure of a workload to be
>   run in Nomad.
>
> 1. an instance of such a workload running in a Nomad cluster.

This section of the documentation will discuss jobs in terms of both of these
definitions and how a specification becomes an instance of a Nomad job.

