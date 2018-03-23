What do I need to know

1. Creation time of the deployment
2. The time at which each allocation had its health set

Cases:

1. The deployment is created but no allocations are created
Deadline is deployment start time + progress deadline
XXX Deadline is first allocation start time + progress deadline

2. Allocations created and some healthy, some unhealthy
Original deadline is deployment start time + progress deadline 
If any alloc goes healthy, the new deadline is its time + progress deadline

---

Ideas:

1. Think I need an option to kill the alloc after it is unhealthy to cause a
   server side restart.

check_restart handles the case of checks but not the case of templates
blocking, downloading an image, etc.

---
Changed Behavior:

1. The deployment will fail if there is resource issues which I super don't
   like.

THIS IS MORE OR LESS NOT AN OPTION
