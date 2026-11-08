-- rule: retell-known
-- severity: warn
-- layer: lint
-- target: act
-- summary: 청자가 이미 믿는 사실을 같은 화자가 같은 말로 다시 알려 준다 (그 사이 청자의 믿음은 변하지 않았다)
--
-- 수정이 남긴 중복(옮긴 대사의 옛 자리)을 겨냥한다. 문학의 되풀이("다시 말씀드리지요")는 말이 달라서 잡지 않는다.
-- 대상: 단언 발화(lies·decrees·sends 제외)의 내용 명제 P 마다 청자(화자 제외) B. sends 는 전령에게 맡기는 전달이라 되풀이가 아니다.
-- 위반: B 의 believes(P, true) 가 같은 화자의 앞선 행위(cause_act, 시점이 확정으로 앞섬)에서 이미 생겼고,
--   두 행위의 인용문이 같거나 어느 한쪽이 비어 있으며(둘 다 있고 다르면 다른 말로 한 번 더 한 것이다),
--   그 뒤 행위 전까지 B 의 믿음이 바뀌지 않았다 — P 의 부정을 믿게 됐거나 P 를 거짓으로 믿게 된 행이 없고,
--   B 가 P 를 의심하는 문장(`doubts` 는 believes 행을 남기지 않아 belief 테이블에서 본다)도 없다.
-- 청중에 아직 모르는 사람이 섞였거나, 같은 화자가 이미 두 번 넘게 같은 말을 한 모티프(치매 노인의 되풀이 등)면 조용하다.
-- 청중에 아직 모르는 사람이 섞였거나, 같은 화자가 이미 두 번 넘게 같은 말을 한 모티프(치매 노인의 되풀이 등)면 조용하다.
-- 작가가 직접 쓴 `believes`(declared)는 출처로 세지 않는다(작품 밖 가정이라 되풀이가 연출일 수 있다).
WITH told AS (
  SELECT DISTINCT c.act_id, c.pt, c.actor_id, au.entity_id AS hearer, c.prop_id, lint_canon($1, c.prop_id) AS cp, a.quote
    FROM act_content c JOIN act a ON a.id = c.act_id JOIN audience au ON au.act_id = c.act_id
   WHERE c.work_id = $1 AND assertive(c.verb) AND c.verb NOT IN ('lies', 'decrees', 'sends')
     AND c.actor_id IS NOT NULL AND au.entity_id <> c.actor_id),
cand AS MATERIALIZED (  -- 싼 조건(같은 화자·청자·명제·인용문)을 먼저 걸러 precedes 호출을 줄인다
  SELECT t.*, k.cause_act, k.from_pt AS k_pt
    FROM told t
    JOIN believes k ON k.work_id = $1 AND k.entity_id = t.hearer AND k.value AND NOT k.declared
                   AND lint_canon($1, k.prop_id) = t.cp AND k.from_pt IS NOT NULL AND k.from_pt <> t.pt
    JOIN act pa ON pa.id = k.cause_act AND pa.resolved_actor_id = t.actor_id
               AND (pa.quote = '' OR t.quote = '' OR pa.quote = t.quote))
SELECT DISTINCT ON (t.act_id, t.hearer, t.cp)
       'act' AS target_kind, t.act_id AS target_id,
       format('%s tells %s %s again; already believed since %s', lint_short(t.actor_id), lint_short(t.hearer), lint_short(t.prop_id), lint_short(t.cause_act)) AS message,
       jsonb_build_object('hearer', t.hearer, 'prop', t.prop_id, 'first', t.cause_act) AS data
  FROM cand t
 WHERE precedes($1, t.k_pt, t.pt) = 'yes'
   AND NOT EXISTS (  -- 새 청자가 섞인 발화(공지·뒤늦은 폭로)는 되풀이가 아니다: 청중 모두가 이미 알아야 한다
   SELECT 1 FROM audience au
    WHERE au.act_id = t.act_id AND au.entity_id <> t.actor_id
      AND NOT EXISTS (SELECT 1 FROM cand u WHERE u.act_id = t.act_id AND u.cp = t.cp AND u.hearer = au.entity_id))
   AND (SELECT count(DISTINCT u.cause_act) FROM cand u WHERE u.act_id = t.act_id AND u.hearer = t.hearer AND u.cp = t.cp) < 2
   AND NOT EXISTS (
   SELECT 1 FROM believes x
    WHERE x.work_id = $1 AND x.entity_id = t.hearer AND x.from_pt IS NOT NULL
      AND ((x.value = false AND lint_canon($1, x.prop_id) = t.cp)
        OR (x.value AND lint_opposite($1, lint_canon($1, x.prop_id), t.cp)))
      AND precedes($1, t.k_pt, x.from_pt) = 'yes' AND precedes($1, x.from_pt, t.pt) <> 'no')
   AND NOT EXISTS (
   SELECT 1 FROM belief d
    WHERE d.work_id = $1 AND d.entity_id = t.hearer AND d.verb = 'doubts' AND d.at_pt IS NOT NULL
      AND lint_canon($1, d.prop_id) = t.cp
      AND precedes($1, t.k_pt, d.at_pt) = 'yes' AND precedes($1, d.at_pt, t.pt) <> 'no')
 ORDER BY t.act_id, t.hearer, t.cp, t.cause_act
