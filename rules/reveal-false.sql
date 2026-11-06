-- rule: reveal-false
-- severity: warn
-- layer: lint
-- target: reveal
-- summary: 거짓으로 선언된 사실을 독자에게 공개한다
--
-- 대상: reveal 문장(대상 종류 reveal = 그 장면 ID, 억제도 같은 키). 한 장면의 같은 사실은 한 번만 낸다.
-- 위반: 공개하는 명제가 작품 안에서 거짓이다 — render_fact_true 가 별칭·부정을 따라가 판정한다
--   (value false 로 선언된 사실, 그 부정의 부정 등). derived·unknown 은 참으로 둔다.
SELECT DISTINCT ON (r.scene_id, r.prop_id) 'reveal' AS target_kind, r.scene_id AS target_id, r.src_id,
       format('reveal of false fact %s', lint_short(r.prop_id)) AS message,
       jsonb_build_object('prop', r.prop_id, 'scene', r.scene_id) AS data
  FROM reveal r
 WHERE r.work_id = $1 AND r.prop_id IS NOT NULL AND NOT render_fact_true($1, r.prop_id)
