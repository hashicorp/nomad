# Quick'N'Dirty Notes from Hack Week

both the sidecar and the allocrunner-driven lock{} go down if the client goes down:
* sidecar: api.sock file goes away with the client agent
* lock{}: the lock-management happens in the client process

maybe that is just fine, and the lock will expire if the timing doesn't work out,
then some other alloc will get the lock as intended.

different degrees of lock guarantees:
* sidecar process writes a file itself into /alloc/ for the main task to read
  * if the sidecar is talking directly to a server, instead of api.sock,
    the sidecar and main task could continue working while client is down.
* sidecar or lock{} writes to the variable, and main task template{}s the var,
  so a change_signal could be set to the main task when the var changes.
* actually hit the lock API in your own code
  * the sidecar could act as a sorta demo or template
