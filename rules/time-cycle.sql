-- rule: time-cycle
-- severity: warn
-- layer: derive
-- target: when
-- summary: the time constraints form a cycle (no schedule satisfies them)
--
-- The verdict is made at load time by solve_time() (db/migrations/002_time.sql): for order cycles (strongly connected components of min ≥ 0 edges) and
-- for each other negative cycle, one run_id 'derive' diagnostic (src = the earliest statement among the cycle edges, data.edges = time_edge.ord).
-- Suppression (`#! allow time-cycle` attached to when/born/dies) removes that edge from the solve and leaves the diagnostic as waived.
-- layer: derive, so the linter does not run this file and shows the rows already in diagnostic as they are.
-- Below is a query that looks at the same rows (for a person using psql).
SELECT target_kind, target_id, message, data
  FROM diagnostic WHERE work_id = $1 AND run_id = 'derive' AND rule_id = 'time-cycle'
