-- 019: reader terms (Plan 15 Task 1). Helpers for `term` declarations (entity kind term, surfaces in name, gloss in attribute)
--   and `render explain <term>` (directive key explain, values = term ids). The verdict is rules/reader-unknown-term.sql.
--   scene_told_ord(w) → (scene_id, ord): narration rank 1..n of shown scenes. scene.discourse_ord is file order only, so
--     `discourse after X` is applied here: in discourse_ord order, each such scene moves to right after X.
--   term_explained_at(w) → (term_id, ord): the first told position whose scene carries `explain` for that term.
--   term_hit(t, surface): the surface occurs in t as a whole word. Before it: start or a non-letter. After it: end or a non-letter,
--     optionally after a run of Korean particles. Letters = Hangul syllables, ASCII letters, digits, _ (so "칩" does not hit "칩거").
--     Particles (fixed list): 은 는 이 가 을 를 에 의 도 와 과 로 으로 에서 에게 만 까지 부터 처럼 보다 이나 나 랑 이랑 께 한테 이다 이야.
-- ponytail: fixed particle list, no morphology. A surface followed by a particle-shaped syllable run (e.g. "칩도로") still hits.
SET LOCAL search_path = public, ag_catalog;

CREATE FUNCTION scene_told_ord(w text) RETURNS TABLE (scene_id text, ord int)
LANGUAGE plpgsql STABLE SET search_path = public, ag_catalog AS $$
DECLARE ids text[]; r record; i int;
BEGIN
  SELECT coalesce(array_agg(s.id ORDER BY s.discourse_ord, s.id), '{}') INTO ids FROM scene s WHERE s.work_id = w AND s.shown;
  FOR r IN SELECT s.id, s.discourse_after AS a FROM scene s
            WHERE s.work_id = w AND s.shown AND s.discourse_after IS NOT NULL AND s.discourse_after <> s.id
            ORDER BY s.discourse_ord, s.id LOOP
    CONTINUE WHEN array_position(ids, r.a) IS NULL;
    ids := array_remove(ids, r.id);
    i := array_position(ids, r.a);
    ids := ids[1:i] || r.id || ids[i + 1:];
  END LOOP;
  RETURN QUERY SELECT u.id, u.n::int FROM unnest(ids) WITH ORDINALITY u(id, n);
END $$;

CREATE FUNCTION term_explained_at(w text) RETURNS TABLE (term_id text, ord int)
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT v, min(o.ord)::int FROM directive d JOIN scene_told_ord(w) o ON o.scene_id = d.scene_id, unnest(d."values") v
   WHERE d.work_id = w AND d.key = 'explain' GROUP BY v $$;

CREATE FUNCTION term_hit(t text, surface text) RETURNS bool
LANGUAGE sql IMMUTABLE AS $$
  SELECT t ~ ('(^|[^가-힣A-Za-z0-9_])' || regexp_replace(surface, '([].^$*+?(){}|\\[-])', '\\\1', 'g')
              || '(은|는|이|가|을|를|에|의|도|와|과|로|으로|에서|에게|만|까지|부터|처럼|보다|이나|나|랑|이랑|께|한테|이다|이야)*($|[^가-힣A-Za-z0-9_])') $$;
