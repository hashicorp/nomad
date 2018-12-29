# QEMU Test Images

## `linux-0.2.img`

via https://en.wikibooks.org/wiki/QEMU/Images

Does not support graceful shutdown.

## Alpine

```
qemu-img create -fmt qcow2 alpine.qcow2 8G

# Download virtual x86_64 Alpine image https://alpinelinux.org/downloads/
qemu-system-x86_64 -cdrom path/to/alpine.iso -hda alpine.qcow2 -boot d -net nic -net user -m 256 -localtime

# In the guest run setup-alpine and exit when complete

# Boot again with:
qemu-system-x86_64 alpine.qcow2
```
