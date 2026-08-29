# The generator polynomial is memoised for the process, not the symbol

`reedsolomon.Encode` takes its Reed-Solomon generator polynomial from a
process-wide table, filled lazily, one `sync.Once` per degree. Every degree
ISO/IEC 18004 table 9 asks for — thirteen of them, 7 to 30, across all 160
version and recovery level combinations — is built at most once for the life of
the program.

## Why not once per symbol

The generator polynomial is a pure function of a block's number of error
correction codewords, and every block of a symbol has the same number. All of
the measured waste is therefore inside one loop: `encodeBlocks` calls
`reedsolomon.Encode` once per block, so a version 40 Highest symbol rebuilt one
identical degree-30 polynomial 81 times, at nearly 1900 allocations each.

Hoisting it out of that loop would recover the same time with no shared state
at all. `(data, numECBytes)` travel together through the loop, which is a type
wanting to be born: a `reedsolomon.Encoder` holding the generator, built once
per symbol and asked for each block. No table, no `sync.Once`, no defensive
copy, no bound.

It was not taken because `Encode` is exported, and this is a fork of a library
with thousands of dependants calling it directly. An encoder value would speed
up our own loop and nothing else; `Encode` would have to carry the memoisation
regardless, and we would then maintain both. The global is real, but narrow:
write-once, unreachable from outside the package, and read on a path that takes
no lock.

## The bound, and the copy

The table stops at degree 30. `Encode` is exported and takes the degree from
its caller, so an unbounded cache would grow on a key the package does not
control, and a fixed array over a small dense key space needs neither hashing
nor a lock. A degree past 30 still encodes — it is built afresh on every call,
as every degree was before the table existed.

`TestMaxErrorCodewordsPerBlock`, in the parent package, pins that bound against
the version table. It lives there because `reedsolomon` cannot see the table —
`qrcode` imports `reedsolomon`, not the other way round — and because a bound
set too low fails no other test in the repository: the encoded output would be
byte-identical, merely slower.

Callers receive a copy rather than the cached polynomial. Nothing downstream
writes to a `gfPoly` today — `gfPolyRemainder`, `gfPolyMultiply` and
`gfPolyAdd` each build a fresh one, and `normalised` only reslices its value
receiver — so the copy is defence, not necessity. It costs one allocation
against the ~1900 it saves, and buys back the freedom to write those three
functions without remembering that one of their arguments is shared with every
later caller.
