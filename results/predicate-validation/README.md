# Finite-domain validation of reported preconditions

For the four harnesses where Zorya attached a path precondition to a race, we
enumerated the declared valid command-line domain and ran each input once under
Go's race detector.

- Gotenberg, gorest and OliveTin: first argument in `{0,1}` and second argument
  in `{0,...,9}` (20 inputs each).
- Mailpit: both arguments in `{0,1}` (4 inputs).

An execution counted as a race when its output contained either
`WARNING: DATA RACE` or Go's concurrent-map failure. The predicate predicted
the race detector's outcome for all 64 inputs: 25 reports inside the predicate,
39 non-reports outside it, and no mismatches. See `summary.tsv`.

This check validates the predicates on these finite harness domains. Because
each input was run once, a non-report does not prove that no other schedule can
race.
