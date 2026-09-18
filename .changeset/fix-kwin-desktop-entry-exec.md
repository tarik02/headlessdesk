---
"headlessdesk": patch
---

Fix KWin screenshot authorization under Nix by pointing the installed desktop
entry's `Exec` at the public `bin/headlessdesk` launcher, which is the name
NixOS-patched KWin matches against `/proc/<pid>/exe`. Also install a
`headlessdesk-wrapped.desktop` entry for unpatched KWin.
