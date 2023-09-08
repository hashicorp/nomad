// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

/*
Package tasklifecycle manages the execution order of tasks based on their
lifecycle configuration. Its main structs are the Coordinator and the Gate.

The Coordinator is used by an allocRunner to signal if a taskRunner is allowed
to start or not. It does so using a set of Gates, each for a given task
lifecycle configuration.

The Gate provides a channel that can be used to block its listener on demand.
This is done by calling the Open() and Close() methods in the Gate which will
cause activate or deactivate a producer at the other end of the channel.

The allocRunner feeds task state updates to the Coordinator that then uses this
information to determine which Gates it should open or close. Each Gate is
connected to a taskRunner with a matching lifecycle configuration.

In the diagrams below, a solid line from a Gate indicates that it's open
(active), while a dashed line indicates that it's closed (inactive). A
taskRunner connected to an open Gate is allowed to run, while one that is
connected to a closed Gate is blocked.

The Open/Close control line represents the Coordinator calling the Open() and
Close() methods of the Gates.

In this state, the Coordinator is allowing prestart tasks to run, while
blocking the main tasks.

	         ┌────────┐
	         │ ALLOC  │
	         │ RUNNER │
	         └───┬────┘
	             │
	         Task state
	             │
	┌────────────▼────────────┐
	│Current state:           │
	│Prestart                 │         ┌─────────────┐
	│                         │         │ TASK RUNNER │
	│     ┌───────────────────┼─────────┤ (Prestart)  │
	│     │                   │         └─────────────┘
	│     │                   │
	│     │                   │         ┌─────────────┐
	│     │ COORDINATOR       │         │ TASK RUNNER │
	│     │             ┌─ ─ ─┼─ ─ ─ ─┬╶┤   (Main)    │
	│     │             ╷     │       ╷ └─────────────┘
	│     │             ╷     │       ╷
	│     │             ╷     │       ╷ ┌─────────────┐
	│   Prestart       Main   │       ╷ │ TASK RUNNER │
	└─────┬─┬───────────┬─┬───┘       └╶┤   (Main)    │
	      │ │Open/      ╷ │Open/        └─────────────┘
	      │ │Close      ╷ │Close
	   ┌──┴─▼─┐      ┌──┴─▼─┐
	   │ GATE │      │ GATE │
	   └──────┘      └──────┘

When the prestart task completes, the allocRunner will send a new batch of task
states to the Coordinator that will cause it to transition to a state where it
will close the Gate for prestart tasks, blocking their execution, and will open
the Gate for main tasks, allowing them to start.

	         ┌────────┐
	         │ ALLOC  │
	         │ RUNNER │
	         └───┬────┘
	             │
	         Task state
	             │
	┌────────────▼────────────┐
	│Current state:           │
	│Main                     │         ┌─────────────┐
	│                         │         │ TASK RUNNER │
	│     ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┼─ ─ ─ ─ ─┤ (Prestart)  │
	│     ╷                   │         └─────────────┘
	│     ╷                   │
	│     ╷                   │         ┌─────────────┐
	│     ╷ COORDINATOR       │         │ TASK RUNNER │
	│     ╷             ┌─────┼───────┬─┤   (Main)    │
	│     ╷             │     │       │ └─────────────┘
	│     ╷             │     │       │
	│     ╷             │     │       │ ┌─────────────┐
	│   Prestart       Main   │       │ │ TASK RUNNER │
	└─────┼─┬───────────┬─┬───┘       └─┤   (Main)    │
	      ╷ │Open/      │ │Open/        └─────────────┘
	      ╷ │Close      │ │Close
	   ┌──┴─▼─┐      ┌──┴─▼─┐
	   │ GATE │      │ GATE │
	   └──────┘      └──────┘

Diagram source:
https://asciiflow.com/#/share/eJyrVspLzE1VssorzcnRUcpJrEwtUrJSqo5RqohRsjI0MDTViVGqBDKNLA2ArJLUihIgJ0ZJAQYeTdmDB8XE5CGrVHD08fF3BjPRZYJC%2Ffxcg7DIEGk6VDWyUEhicbZCcUliSSp2hfgNR6BpxCmDmelcWlSUmlcCsdkKm62%2BiZmo7kEOCOK8jtVmrGZiMVchxDHYGzXEYSpIspVUpKAREOQaHOIYFKKpgGkvjcIDp8kk2t7zaEoDcWgCmsnO%2Fv5BLp5%2BjiH%2BQVhNbkKLjyY8LtNFAyDdCgoavo6efppQ0%2FDorkETrQGypxDtrxmkmEyiK8iJ24CiVGAeKyqBGgPNVWjmYk%2FrVE7X8LhBiwtEcQRSBcT%2B%2Bs4KyK5D4pOewlFMRglfuDy6vmkoLoaL1yDLwXUquDuGuCogq4aLYDd9CnbT0V2uVKtUCwCqNQgp)
*/
package tasklifecycle
