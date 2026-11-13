# readcheck

Model-reader gate (`tools/readcheck`). Per episode, with fixed thresholds: first-time-reader structural confusions <= 1, term confusions == 0, and the episode wins at least 1 of 2 order-swapped pairwise comparisons against `ref-unsu.txt`. vocab and artifact confusions are ignored; `continue` is recorded only. Call budget defaults to files x 3 (a retry on unparsable JSON counts); an episode that cannot start within the budget is `SKIPPED(budget)`; any non-PASS makes the exit code 1. Raw responses are saved as `<file>.readcheck.json`.

`ref-unsu.txt`: first ~2,000 characters of the story body of 현진건 "운수 좋은 날" (1924, public domain), from `data/raw/운수-좋은-날.txt` (ko.wikisource), header and marker removed, `( )` blanks removed.
