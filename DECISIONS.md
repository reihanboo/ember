Config defaults leave build/run commands and environment empty; other defaults follow the config reference.
Watcher errors emit a synthetic Change with nil Paths to request a full rebuild.
Partial process output lines receive a newline when flushed after process exit.
Stop timeout is the graceful period; cancellation skips grace, but forced cleanup waits for the tree.
Linux abrupt-parent cleanup tests verify the direct child; grandchild cleanup depends on Registry shutdown.
Output paths start at app-1 per process; Next returns paths without creating .ember/bin.
Prune preserves .pdb/.ilk siblings for kept outputs along with the output itself.
Supervisor events use change, control, key, child-exit, and build-finish values; invalid states stringify as Unknown.
The event-loop skeleton logs unhandled events and injects builder, runner, clock, and logger interfaces.
Build triggers receive a copied build.Spec with {out} substituted; a new trigger cancels and supersedes an active build.
Control responses use a dot terminator line and dot-stuff payload lines beginning with a dot.
