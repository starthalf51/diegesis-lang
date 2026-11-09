-- 005: judgment helpers shared by the lint rules (rules/*.sql). Each rule's meaning is in its own file header.
SET LOCAL search_path = public, ag_catalog;

-- Canonical proposition: Q if it is an alias (`fact x = Q`), otherwise itself. ponytail: resolves one level only (an alias of an alias stays).
CREATE FUNCTION lint_canon(w text, p text) RETURNS text
LANGUAGE sql STABLE SET search_path = public, ag_catalog
  AS $$ SELECT coalesce((SELECT def->>'alias' FROM fact WHERE work_id = w AND id = p), p) $$;

-- e and the groups e (indirectly) belongs to. A group's beliefs and exemptions are inherited by its members.
CREATE FUNCTION lint_self_and_groups(w text, e text) RETURNS TABLE (id text)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH RECURSIVE up(id) AS (
    SELECT e UNION SELECT g.group_id FROM group_member g JOIN up ON g.member_id = up.id WHERE g.work_id = w)
  SELECT id FROM up $$;

-- Information-flow exemption: self or a group it belongs to has knows oracle|omniscient.
CREATE FUNCTION lint_exempt(w text, e text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM entity x JOIN lint_self_and_groups(w, e) u ON u.id = x.id
                  WHERE x.work_id = w AND x.knows IN ('oracle', 'omniscient')) $$;

-- Whether two canonical propositions negate each other (`not P` and P). Arguments are IDs that went through lint_canon.
CREATE FUNCTION lint_opposite(w text, a text, b text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM fact f WHERE f.work_id = w AND f.def ? 'not'
                    AND ((f.id = a AND lint_canon(w, f.def->>'not') = b)
                      OR (f.id = b AND lint_canon(w, f.def->>'not') = a))) $$;

-- How well e knows canonical proposition c at point at: 2 = certain, 1 = only sources with undetermined order (maybe), 0 = no source.
-- Sources = believes(P, true) of e or of a group e belongs to, plus their decrees P (performatives: whoever proclaims knows).
-- Source point NULL = from the start (background knowledge, believes in top-level/declaration blocks) → certain. Otherwise precedes(source, at).
-- If the source point equals at (the same act), count it only when it is a belief the author declared (declared).
CREATE FUNCTION lint_known(w text, e text, c text, at text) RETURNS int
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH src(pt, declared) AS (
    SELECT b.from_pt, b.declared FROM believes b JOIN lint_self_and_groups(w, e) u ON u.id = b.entity_id
     WHERE b.work_id = w AND b.value AND lint_canon(w, b.prop_id) = c
    UNION ALL
    SELECT x.pt, false FROM act_content x JOIN lint_self_and_groups(w, e) u ON u.id = x.actor_id
     WHERE x.work_id = w AND x.verb = 'decrees' AND lint_canon(w, x.prop_id) = c)
  SELECT coalesce(max(CASE WHEN pt IS NULL THEN 2
                           ELSE CASE precedes(w, pt, at) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END), 0)
    FROM src WHERE pt IS DISTINCT FROM at OR declared $$;

-- Short name for diagnostic messages: 'w/mod/x' → 'x', 'w/holds(w/m/r,w/m/x)' → 'holds(r,x)'.
CREATE FUNCTION lint_short(x text) RETURNS text LANGUAGE sql IMMUTABLE
  AS $$ SELECT regexp_replace(x, '[^(), ]*/', '', 'g') $$;
