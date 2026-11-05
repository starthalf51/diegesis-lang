-- 006: 의심이 들은 믿음을 거둔다(Task 9). 003 의 derive_beliefs 와 005 의 lint_known 을 바꾼다.
--   `X doubts P` (시점 t) — X 가 t 이전(t 와 같은 시점 포함)에 들어서 얻은 P 의 믿음 행은 t 부터 거둔다:
--   그 행의 until_pt = t(그런 의심 중 가장 이른 것). 들은 믿음 = 단언 발화(assertive, shows 제외)의 청중 행.
--   지각(sees)·읽기(reads)·명시적 believes·배경 지식은 거두지 않는다. t 뒤의 새 출처는 새 행이라 다시 믿는다.
--   배경 doubts(시점 없음)는 행을 만들지 않고 아무것도 거두지 않는다 — 처음부터 믿지 않는다는 뜻 그대로다.
--   ponytail: 의심한 사람 자신의 행만 거둔다(그룹이 들은 것은 그대로). 그룹 의심이 필요해지면 lint_self_and_groups 로 넓힌다.
SET LOCAL search_path = public, ag_catalog;

ALTER TABLE believes ADD COLUMN until_pt text;

CREATE OR REPLACE FUNCTION derive_beliefs(w text) RETURNS void
LANGUAGE plpgsql SET search_path = public, ag_catalog AS $$
BEGIN
  DELETE FROM believes WHERE work_id = w;
  DELETE FROM uses WHERE work_id = w;
  DELETE FROM deception WHERE work_id = w;

  INSERT INTO believes(work_id, entity_id, prop_id, value, from_pt, cause_act, declared, src_id)
  SELECT w, au.entity_id, c.prop_id, true, c.pt, c.act_id, false, c.src_id
    FROM act_content c JOIN audience au ON au.act_id = c.act_id
   WHERE c.work_id = w AND assertive(c.verb) AND au.entity_id IS DISTINCT FROM c.actor_id
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, false, c.pt, c.act_id, false, c.src_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, c.actor_id, c.prop_id, true, c.pt, c.act_id, false, c.src_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'sees' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, true, a.pt, a.id, false, a.src_id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'reads' AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT w, b.entity_id, b.prop_id, true, b.at_pt, (SELECT id FROM act WHERE id = b.because_pt), true, b.src_id
    FROM belief b WHERE b.work_id = w AND b.verb = 'believes' AND b.prop_id IS NOT NULL;

  -- 의심: 들은 행(청중 행, 화자 아님, 선언 아님)마다 그 뒤(precedes yes, 같은 시점 포함)의 가장 이른 의심.
  UPDATE believes bl SET until_pt = (
      SELECT b.at_pt FROM belief b JOIN point p ON p.id = b.at_pt
       WHERE b.work_id = w AND b.verb = 'doubts' AND b.entity_id = bl.entity_id AND b.at_pt IS NOT NULL
         AND lint_canon(w, b.prop_id) = lint_canon(w, bl.prop_id) AND precedes(w, bl.from_pt, b.at_pt) = 'yes'
       ORDER BY p.earliest, b.src_id LIMIT 1)
    FROM act s
   WHERE bl.work_id = w AND bl.value AND NOT bl.declared AND s.id = bl.cause_act
     AND assertive(s.verb) AND s.verb <> 'shows' AND s.resolved_actor_id IS DISTINCT FROM bl.entity_id;

  INSERT INTO uses(work_id, entity_id, prop_id, at_pt, act_id)
  SELECT w, c.actor_id, c.prop_id, c.pt, c.act_id
    FROM act_content c WHERE c.work_id = w AND assertive(c.verb) AND c.verb <> 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, p, a.pt, a.id
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'writes' AND a.resolved_actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.resolved_actor_id, a.because_id, a.pt, a.id
    FROM act a WHERE a.work_id = w AND a.because_id IS NOT NULL AND a.resolved_actor_id IS NOT NULL;

  INSERT INTO deception(work_id, act_id, liar_id, prop_id)
  SELECT w, c.act_id, c.actor_id, c.prop_id
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL;
END $$;

-- lint_known(005)에 거둠을 더한다: until_pt 가 at 보다 확정적으로 앞이면(같은 시점이 아님) 그 행은 출처가 아니다.
-- 같은 시점은 의심이 그 행위 뒤에 적힌 경우다(문장 시점 = 직전 행위 시점) → 그 행위 때는 아직 믿었다.
-- 선후가 maybe 이면 확정이 아니므로 그 행은 많아야 1(info)로 센다.
CREATE OR REPLACE FUNCTION lint_known(w text, e text, c text, at text) RETURNS int
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  WITH src(pt, declared, until_pt) AS (
    SELECT b.from_pt, b.declared, b.until_pt FROM believes b JOIN lint_self_and_groups(w, e) u ON u.id = b.entity_id
     WHERE b.work_id = w AND b.value AND lint_canon(w, b.prop_id) = c
    UNION ALL
    SELECT x.pt, false, NULL FROM act_content x JOIN lint_self_and_groups(w, e) u ON u.id = x.actor_id
     WHERE x.work_id = w AND x.verb = 'decrees' AND lint_canon(w, x.prop_id) = c),
  graded(g, cut) AS (
    SELECT CASE WHEN pt IS NULL THEN 2
                ELSE CASE precedes(w, pt, at) WHEN 'yes' THEN 2 WHEN 'maybe' THEN 1 ELSE 0 END END,
           CASE WHEN until_pt IS NULL OR until_pt = at THEN 2
                ELSE CASE precedes(w, until_pt, at) WHEN 'yes' THEN 0 WHEN 'maybe' THEN 1 ELSE 2 END END
      FROM src WHERE pt IS DISTINCT FROM at OR declared)
  SELECT coalesce(max(least(g, cut)), 0) FROM graded $$;
