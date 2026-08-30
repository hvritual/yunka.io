# `pkg/fastfeedback`

`pkg/fastfeedback` owns disposable, deterministic evidence used by C11.4 fast-feedback optimizations.

It does not compile contracts, validate modules, compile assembly, decide authorization/transactions, or own runtime state. A cache hit is an optimization precondition only; canonical full generation/check remains authoritative.

The package deliberately stores no timestamps or absolute host paths. Unverified/dirty engine identities are never reusable.
