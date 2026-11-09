-- rule: inferred-deception
-- severity: info
-- layer: lint
-- target: act
-- summary: 쓴 사람이 거짓인 줄 알고 쓴 것으로 판정된 문서(007) — 작가의 실수인지 확인하라
--
-- Target: `X writes art` that has a deception row (007: the writer definitely knows ¬P at the time of writing). One finding per document,
--   listing every proposition seen as false. Unlike lies, the author did not write it as a lie but the engine inferred it, so it is shown.
--   If it is a genuine forged letter, leave it as is (info, not a false positive). If it is the author's mistake (the writer cannot have known ¬P), fix the source.
SELECT 'act' AS target_kind, a.id AS target_id,
       format('%s writes %s knowing it false (inferred deception: %s)', lint_short(a.resolved_actor_id),
              string_agg(DISTINCT lint_short(art.id), ', '), string_agg(DISTINCT lint_short(d.prop_id), ', ')) AS message,
       jsonb_build_object('entity', a.resolved_actor_id, 'props', jsonb_agg(DISTINCT d.prop_id)) AS data
  FROM act a JOIN deception d ON d.act_id = a.id
  JOIN artifact art ON art.id = ANY (a.objects) AND d.prop_id = ANY (art.contains)
 WHERE a.work_id = $1 AND a.verb = 'writes'
 GROUP BY a.id, a.resolved_actor_id
