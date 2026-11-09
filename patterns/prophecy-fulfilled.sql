-- pattern: prophecy-fulfilled
-- layer: foreshadow
-- summary: a proposition asserted by a prophet (knows oracle) was not true or was undetermined at the time, and later becomes true
--
-- Prophecy = proposition P ≡ (base b = v) conveyed by an assertion (not a lie) of a prophet (self or group has knows oracle).
-- Not true at the time (or undetermined): one of the following. A fact with no point (not a state predicate) is judged only by the second condition
--   (a declared truth value is independent of the point, so it cannot tell us "not true then").
--   - b is a state predicate (holds·alive) and truth_at(b) ≠ v at that point (false or not determined)
--   - no one in the audience knows P at that point (char_knows ≠ yes: undetermined to the listener. The coder type — already true but nobody knows)
-- Later becomes true (route; any one):
--   state     the state predicate is certainly v at the end of a visible scene after the prophecy (role holding interval / birth·death point)
--   confirmed a non-prophet asserts P after the prophecy, truth_at = v at that point, none of that person's beliefs in P
--             came from the prophet's act, and that assertion's audience overlaps the prophecy's audience (confirming to someone who heard the prophecy —
--             repeating the prophecy, or revealing a past fact among people unrelated to the prophecy, is not confirmation)
--   payoff    in the same scene as the prophecy, P is set up as a promise setup and paid off after the prophecy (the realization the author wrote)
-- Target = the prophecy act. span = prophecy position ~ earliest realization position (NULL if in an unseen scene).
WITH proph AS (
  SELECT a.* FROM act_props($1) a
   WHERE assertive(a.verb) AND a.verb <> 'lies' AND is_oracle($1, a.actor_id)),
notyet AS (
  SELECT p.* FROM proph p
   WHERE (fact_is_state($1, p.base) AND truth_at($1, p.base, p.pt) IS DISTINCT FROM p.val)
      OR NOT EXISTS (SELECT 1 FROM audience au WHERE au.act_id = p.act_id AND au.entity_id IS DISTINCT FROM p.actor_id
                        AND char_knows($1, au.entity_id, p.base, p.val, p.pt) = 'yes')),
f AS (
  SELECT n.act_id, n.prop_id, 'state' AS route, min(s.discourse_ord) AS ord, NULL::text AS by_act
    FROM notyet n JOIN scene s ON s.work_id = $1 AND s.shown
   WHERE fact_is_state($1, n.base) AND truth_at($1, n.base, n.pt) IS DISTINCT FROM n.val
     AND precedes($1, n.pt, s.end_pt) = 'yes' AND truth_at($1, n.base, s.end_pt) = n.val
   GROUP BY n.act_id, n.prop_id
  UNION ALL
  SELECT n.act_id, n.prop_id, 'confirmed', b.ord, b.act_id
    FROM notyet n JOIN act_props($1) b ON b.base = n.base AND b.val = n.val AND b.act_id <> n.act_id
   WHERE assertive(b.verb) AND b.verb <> 'lies' AND NOT is_oracle($1, b.actor_id)
     AND precedes($1, n.pt, b.pt) = 'yes' AND precedes($1, b.pt, n.pt) <> 'yes'
     AND truth_at($1, b.base, b.pt) = b.val
     AND NOT EXISTS (SELECT 1 FROM believes x JOIN act y ON y.id = x.cause_act
                       CROSS JOIN LATERAL prop_norm($1, x.prop_id) z
                      WHERE x.work_id = $1 AND x.entity_id = b.actor_id AND z.base = b.base
                        AND is_oracle($1, y.resolved_actor_id))
     AND EXISTS (SELECT 1 FROM audience pa JOIN audience ca ON ca.entity_id = pa.entity_id
                  WHERE pa.act_id = n.act_id AND ca.act_id = b.act_id)
  UNION ALL
  SELECT n.act_id, n.prop_id, 'payoff', sp.discourse_ord, NULL
    FROM notyet n
    JOIN setup st ON st.work_id = $1 AND st.kind = 'promise' AND st.scene_id = n.scene_id
    CROSS JOIN LATERAL prop_norm($1, st.prop_id) z
    JOIN payoff po ON po.work_id = $1 AND po.setup_id = st.id
    LEFT JOIN scene sp ON sp.id = po.scene_id AND sp.shown
   WHERE z.base = n.base AND z.val = n.val AND po.at_pt IS NOT NULL AND precedes($1, n.pt, po.at_pt) = 'yes')
SELECT 'act' AS target_kind, n.act_id AS target_id, n.ord AS span_from, min(f.ord) AS span_to,
       jsonb_build_object('prop', n.prop_id, 'oracle', n.actor_id,
                          'fulfilled', jsonb_agg(jsonb_build_object('route', f.route, 'ord', f.ord, 'act', f.by_act)
                                                 ORDER BY f.ord NULLS LAST, f.route)) AS data
  FROM notyet n JOIN f ON f.act_id = n.act_id AND f.prop_id = n.prop_id
 GROUP BY n.act_id, n.prop_id, n.ord, n.actor_id
