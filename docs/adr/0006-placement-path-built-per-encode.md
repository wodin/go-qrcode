# The placement path is built per encode, not memoised for the process

`encode()` builds one `placementPath` before its eight-mask loop and hands it
to every mask. Nothing survives the call: no package-level table, no
`sync.Once`. The next encode of the same version walks the data region again.

## Why not once per mask

The path a version's data region is filled in is fixed by the version alone —
the function patterns it steps over do not move for the content, the recovery
level or the data mask. `addData` nevertheless walked it from the symbol it
was filling, so a version 40 encode built the same 29,648-module path eight
times, 463 KiB each. Building it once removes seven of the eight:

    BenchmarkEncodeMaximumSize   7.522m -> 6.122m   -18.6%
    B/op                        5.14MiB -> 2.04MiB  -60.3%

It costs one function pattern symbol per encode, ~356 allocations and 78 KB,
because `newPlacementPath` builds its own rather than borrowing the one a mask
is about to build. That is the 2% rise in allocation count against a 60% fall
in allocated bytes.

## Why not once per process

A forty-entry table keyed on the version number would go further, and was
measured:

    BenchmarkEncodeMaximumSize   6.122m -> 5.896m   -3.7%

That further 3.7% is mostly an artefact of how the benchmark is shaped.
`BenchmarkEncodeMaximumSize` encodes one version 200 times in one process, so
a table amortises a single walk across all 200. A program that encodes one
symbol — including this repository's own CLI, which calls `New` exactly once —
gets the whole of the 18.6% from the per-encode build and nothing measurable
from the table.

What the table would cost is not conditional in the same way. It retains its
entries for the life of the process: 463 KiB for version 40 alone, 6.74 MiB if
a program touches all forty. Any importer that encodes once holds that memory
until it exits, whether or not it ever encodes again.

## Why ADR-0005 does not decide this

ADR-0005 took a process-wide table for the Reed-Solomon generator polynomial,
which looks like the same trade answered the other way. It is not.

The generator polynomial is built inside `reedsolomon.Encode`, which is
exported, and this is a fork of a library with many direct dependants. Per
symbol memoisation would have meant a new exported type that speeds up our own
loop and no one else's, leaving `Encode` to carry a cache regardless — two
mechanisms where one was needed. The lifetime was forced by the API boundary.

The placement path has no such boundary. Every route to it — `encode()` and
`newCodewordLayout` — is inside this package and ours to restructure, so the
narrowest lifetime that removes the waste is available, and is what we take.

## If this is ever memoised anyway

Cache the modules, keyed on the version number. Never cache `placementPath`
values.

A `placementPath` holds a `qrCodeVersion`, and a `qrCodeVersion` carries the
recovery level. The modules do not depend on the level — it changes the values
of the format info modules, never their positions — but `formatInfo` reads the
level, so a table of `placementPath` values keyed on the version number would
hand every caller the recovery level of whichever caller arrived first. Every
other level would get the wrong format info: a symbol that is silently
mis-encoded rather than one that fails.
