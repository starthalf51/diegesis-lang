-- 021: render_scene re-explain fix (Plan 15 Task 3, from Task 6). terms_new = every term with an `explain` directive in this scene,
--   even if an earlier scene explained it too; terms_known = terms first explained at an earlier told rank and not in terms_new.
--   Same body as 020 except the two term queries. Order: first explanation, then id.
SET LOCAL search_path = public, ag_catalog;

CREATE OR REPLACE FUNCTION render_scene(w text, s text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SET search_path = public, ag_catalog AS $$
DECLARE
  sc scene; n int; pov_v text; pov text; acts jsonb; beats jsonb; cast_ids text[]; facts text[]; held text[];
  facts_c text[]; held_c text[]; -- canonical propositions (for comparison)
  tn jsonb; tk jsonb; here text[];
BEGIN
  SELECT count(*) INTO n FROM scene x WHERE x.work_id = w AND (x.id = s OR lint_short(x.id) = s);
  IF n <> 1 THEN
    RAISE EXCEPTION 'render_scene: % scenes named % in %', n, s, w;
  END IF;
  SELECT * INTO sc FROM scene x WHERE x.work_id = w AND (x.id = s OR lint_short(x.id) = s);

  -- Line up acts (k = index within the scene) and entries/exits in one sequence: (gap g, sub) — exit 0, entry 1, act 2.
  CREATE TEMP TABLE IF NOT EXISTS render_row (g int, sub int, beat text, act jsonb, ents text[], fact text) ON COMMIT DROP;
  DELETE FROM render_row;
  CREATE TEMP TABLE IF NOT EXISTS render_act ON COMMIT DROP AS SELECT a.*, 0::bigint AS k FROM act a LIMIT 0;
  DELETE FROM render_act;
  INSERT INTO render_act SELECT a.*, row_number() OVER (ORDER BY a.ord) FROM act a WHERE a.work_id = w AND a.scene_id = sc.id;
  SELECT count(*) INTO n FROM render_act;

  INSERT INTO render_row
  SELECT a.k, 2, a.beat_id,
         jsonb_build_object('actor', lint_short(coalesce(a.resolved_actor_id, a.actor_id)), 'verb', a.verb,
           'targets', to_jsonb(ARRAY(SELECT lint_short(t) FROM unnest(a.objects || a."to") WITH ORDINALITY u(t, o)
                                      WHERE EXISTS (SELECT 1 FROM entity e WHERE e.id = u.t AND e.kind <> 'place') ORDER BY o)),
           'quote', a.quote,
           'action', trim(replace(a.action, '_', ' ') || ' ' || coalesce((SELECT string_agg(render_name(w, art.id, 'en'), ', ')
                                                                          FROM artifact art WHERE art.id = ANY (a.objects)), ''))),
         ARRAY[coalesce(a.resolved_actor_id, a.actor_id)] || ARRAY(SELECT t FROM unnest(a.objects || a."to") t
                                                              WHERE EXISTS (SELECT 1 FROM entity e WHERE e.id = t AND e.kind <> 'place')),
         coalesce(a.prop_id, (SELECT art.contains[1] FROM artifact art
                               WHERE art.id = ANY (a.objects) AND a.verb IN ('reads', 'writes') LIMIT 1))
    FROM render_act a;

  INSERT INTO render_row
  SELECT e.g, e.sub, (SELECT r.beat FROM render_act ra JOIN render_row r ON r.g = ra.k AND r.sub = 2
                       WHERE ra.k = least(e.g, n)),
         jsonb_build_object('actor', '', 'verb', e.verb, 'targets', to_jsonb(array_agg(lint_short(e.entity_id) ORDER BY e.entity_id))),
         array_agg(e.entity_id), NULL
    FROM (SELECT 'enter' AS verb, 1 AS sub, p.entity_id,
                 coalesce((SELECT ra.k + 1 FROM render_act ra WHERE ra.id = p.from_pt),
                          (SELECT min(ra.k) FROM render_act ra WHERE ra.beat_id = p.from_pt), 1) AS g
            FROM presence p WHERE p.work_id = w AND p.scene_id = sc.id AND p.from_pt IS DISTINCT FROM sc.start_pt
          UNION ALL
          SELECT 'exit', 0, p.entity_id,
                 coalesce((SELECT ra.k FROM render_act ra WHERE ra.id = p.to_pt),
                          (SELECT min(ra.k) FROM render_act ra WHERE ra.beat_id = p.to_pt), n + 1)
            FROM presence p WHERE p.work_id = w AND p.scene_id = sc.id) e
   GROUP BY e.g, e.sub, e.verb;

  -- POV character: the first value of the pov directive (short name).
  SELECT d."values"[1] INTO pov_v FROM directive d WHERE d.work_id = w AND d.scene_id = sc.id AND d.key = 'pov' LIMIT 1;
  SELECT e.id INTO pov FROM entity e WHERE e.work_id = w AND lint_short(e.id) = pov_v ORDER BY e.id LIMIT 1;

  SELECT array_agg(DISTINCT x) INTO cast_ids FROM (
    SELECT unnest(r.ents) x FROM render_row r UNION SELECT p.entity_id FROM presence p WHERE p.scene_id = sc.id UNION SELECT pov) c
   WHERE x IS NOT NULL;
  SELECT coalesce(array_agg(DISTINCT r.fact) FILTER (WHERE r.fact IS NOT NULL), '{}') INTO facts FROM render_row r;
  SELECT coalesce(array_agg(lint_canon(w, f)), '{}') INTO facts_c FROM unnest(facts) f;

  SELECT coalesce(array_agg(DISTINCT h.p), '{}') INTO held FROM (
    SELECT wh.prop_id p, lint_canon(w, wh.prop_id) c, wh.until_pt, su.discourse_ord u_ord, sd.discourse_ord d_ord
      FROM withhold wh
      LEFT JOIN scene sd ON sd.id = wh.scene_id
      LEFT JOIN scene su ON su.work_id = w AND (su.start_pt = wh.until_pt OR starts_with(wh.until_pt, su.id || '#'))
     WHERE wh.work_id = w AND wh.prop_id IS NOT NULL) h
   WHERE (h.d_ord IS NULL OR h.d_ord <= sc.discourse_ord)
     AND (h.until_pt IS NULL OR h.u_ord > sc.discourse_ord)
     AND NOT h.c = ANY (facts_c)
     AND NOT EXISTS (SELECT 1 FROM reveal r JOIN scene sr ON sr.id = r.scene_id
                      WHERE r.work_id = w AND lint_canon(w, r.prop_id) = h.c AND sr.discourse_ord < sc.discourse_ord);
  SELECT coalesce(array_agg(lint_canon(w, f)), '{}') INTO held_c FROM unnest(held) f;

  -- Beat: one per run (island) where the same beat ID continues.
  SELECT jsonb_agg(jsonb_build_object('id', coalesce(substring(b.beat from '#(.*)$'), ''), 'acts', b.acts) ORDER BY b.first) INTO beats FROM (
    SELECT i.beat, min(i.pos) first, jsonb_agg(i.act ORDER BY i.pos) acts FROM (
      SELECT r.beat, r.pos, r.act || jsonb_build_object('fact', coalesce(lint_short(r.fact), '')) act,
             r.pos - row_number() OVER (PARTITION BY r.beat ORDER BY r.pos) isl
        FROM (SELECT x.*, row_number() OVER (ORDER BY x.g, x.sub) pos FROM render_row x) r) i
     GROUP BY i.beat, i.isl) b;

  SELECT coalesce(array_agg(v), '{}') INTO here FROM directive d, unnest(d."values") v
   WHERE d.work_id = w AND d.scene_id = sc.id AND d.key = 'explain';

  SELECT coalesce(jsonb_agg(jsonb_build_object('surface', render_name(w, t.term_id, 'en'), 'gloss', coalesce(g.value, '')) ORDER BY t.ord, t.term_id) FILTER (WHERE t.term_id = ANY (here)), '[]'),
         coalesce(jsonb_agg(jsonb_build_object('surface', render_name(w, t.term_id, 'en'), 'gloss', coalesce(g.value, '')) ORDER BY t.ord, t.term_id) FILTER (WHERE t.ord < so.ord AND NOT t.term_id = ANY (here)), '[]')
    INTO tn, tk
    FROM scene_told_ord(w) so, term_explained_at(w) t
    LEFT JOIN attribute g ON g.id = t.term_id || '#gloss'
   WHERE so.scene_id = sc.id AND EXISTS (SELECT 1 FROM entity e WHERE e.id = t.term_id AND e.kind = 'term');

  RETURN jsonb_build_object(
    'id', lint_short(sc.id),
    'title', coalesce(nullif(sc.title, ''), lint_short(sc.id)),
    'place', coalesce(render_name(w, sc.place_id, 'en'), ''),
    'previously', coalesce((
      SELECT string_agg(t.line, ' ' ORDER BY t.ord) FROM (
        SELECT s2.discourse_ord ord, coalesce(nullif(s2.title, ''), 'Scene ' || lint_short(s2.id)) || ': ' ||
               coalesce((SELECT string_agg(DISTINCT render_fact_text(w, c.prop_id), '; ')
                           FROM act_content c JOIN act a ON a.id = c.act_id
                          WHERE a.scene_id = s2.id AND NOT lint_canon(w, c.prop_id) = ANY (held_c)), '(no facts)') || '.' line
          FROM scene s2 WHERE s2.work_id = w AND s2.shown AND sc.shown AND s2.discourse_ord < sc.discourse_ord
         ORDER BY s2.discourse_ord DESC LIMIT 3) t), ''),
    'cast', coalesce((SELECT jsonb_agg(jsonb_build_object('id', lint_short(x), 'names', coalesce(
               (SELECT jsonb_object_agg(l.lang, l.texts) FROM (SELECT nm.lang, jsonb_agg(nm.text ORDER BY nm.ord) texts
                  FROM name nm WHERE nm.work_id = w AND nm.owner_id = x AND nm.lang <> '' GROUP BY nm.lang) l),
               jsonb_build_object('en', jsonb_build_array(render_name(w, x, 'en'))))) ORDER BY x)
       FROM unnest(cast_ids) x), '[]'),
    'on_stage', to_jsonb(ARRAY(SELECT lint_short(x) FROM unnest(cast_ids) x
       WHERE EXISTS (SELECT 1 FROM presence p WHERE p.scene_id = sc.id AND p.entity_id = x AND p.from_pt = sc.start_pt)
          OR NOT EXISTS (SELECT 1 FROM presence p WHERE p.scene_id = sc.id AND p.entity_id = x) ORDER BY x)),
    'facts', coalesce((SELECT jsonb_agg(jsonb_build_object('id', lint_short(f), 'text', render_fact_text(w, f),
                                                          'true', render_fact_true(w, f))
                                       || CASE WHEN lint_canon(w, f) <> f
                                               THEN jsonb_build_object('canon', lint_short(lint_canon(w, f))) ELSE '{}' END
                                       ORDER BY f)
       FROM (SELECT DISTINCT unnest(facts || held) f) u), '[]'),
    'beats', coalesce(beats, '[]'),
    'directives', jsonb_build_object(
       'pov', coalesce(lint_short(pov), ''),
       'pace', coalesce((SELECT d."values"[1] FROM directive d WHERE d.scene_id = sc.id AND d.key = 'pace' LIMIT 1), ''),
       'tension', coalesce((SELECT d."values"[1]::float8 FROM directive d WHERE d.scene_id = sc.id AND d.key = 'tension'
                              AND d."values"[1] ~ '^[0-9.]+$' LIMIT 1), 0),
       'focus', coalesce((SELECT to_jsonb(d."values") FROM directive d WHERE d.scene_id = sc.id AND d.key = 'focus' LIMIT 1), '[]'),
       'omit', coalesce((SELECT to_jsonb(d."values") FROM directive d WHERE d.scene_id = sc.id AND d.key = 'omit' LIMIT 1), '[]')),
    'pov_beliefs', to_jsonb(ARRAY(SELECT lint_short(f) FROM unnest(facts) f
       WHERE pov IS NOT NULL AND lint_known(w, pov, lint_canon(w, f), sc.start_pt) = 2 ORDER BY f)),
    'reader_knows', to_jsonb(ARRAY(SELECT lint_short(f) FROM unnest(facts) f, LATERAL (SELECT lint_canon(w, f) fc) z
     WHERE sc.shown AND (
       EXISTS (SELECT 1 FROM act a JOIN scene s2 ON s2.id = a.scene_id
                WHERE a.work_id = w AND s2.shown AND s2.discourse_ord < sc.discourse_ord
                  AND (lint_canon(w, a.prop_id) = z.fc OR z.fc IN (SELECT lint_canon(w, p) FROM unnest(a."with") p)
                       OR (a.verb = 'reads' AND z.fc IN (SELECT lint_canon(w, p) FROM artifact art, unnest(art.contains) p
                                                          WHERE art.id = ANY (a.objects)))))
       OR EXISTS (SELECT 1 FROM reveal r JOIN scene sr ON sr.id = r.scene_id
                   WHERE r.work_id = w AND lint_canon(w, r.prop_id) = z.fc AND sr.discourse_ord < sc.discourse_ord)) ORDER BY f)),
    'withheld', to_jsonb(ARRAY(SELECT lint_short(f) FROM unnest(held) f ORDER BY f)),
    'terms_new', coalesce(tn, '[]'),
    'terms_known', coalesce(tk, '[]'),
    'narration', coalesce((SELECT d."values"[1] FROM directive d WHERE d.scene_id = sc.id AND d.key = 'narration'
                             AND d."values"[1] IN ('first-person', 'third-person') LIMIT 1), ''));
END $$;
