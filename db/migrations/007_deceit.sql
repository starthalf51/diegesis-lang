-- 007: 글로 한 거짓말과 읽은 문서의 의심(Task 9, 적벽).
--   1. 읽은 문서도 증언이다: `X doubts P` 는 그 앞에 읽은(reads) 문서에서 얻은 P 도 거둔다(006 의 들은 믿음과 같게).
--      근거: 『삼국지연의』 47.14-16 조조는 황개의 서신을 읽고 곧 "personal injury trick" 이라며 의심한다.
--   2. 쓴 사람이 쓰는 시점에 ¬P 를 확정적으로 알면(lint_known = 2) 그 문서의 P 는 거짓말이다: deception 행.
--      letter-unknown-info·belief-contradiction 은 그런 (행위, 명제)를 내지 않는다(lies 가 info-flow 에서 빠지는 것과 같다).
--      근거: 45.115 주유가 꾸민 채모·장윤의 서신, 47.14 황개의 거짓 항복 서신.
SET LOCAL search_path = public, ag_catalog;

-- e 가 시점 at 에 P 의 반대(¬P, 별칭 포함)를 확정적으로 아는가.
CREATE FUNCTION lint_knows_false(w text, e text, p text, at text) RETURNS bool
LANGUAGE sql STABLE SET search_path = public, ag_catalog AS $$
  SELECT EXISTS (SELECT 1 FROM fact f WHERE f.work_id = w
                    AND lint_opposite(w, lint_canon(w, f.id), lint_canon(w, p))
                    AND lint_known(w, e, lint_canon(w, f.id), at) = 2) $$;

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

  -- 의심: 들은 행(청중 행, 화자 아님, 선언 아님)과 읽은 행마다 그 뒤(precedes yes, 같은 시점 포함)의 가장 이른 의심.
  UPDATE believes bl SET until_pt = (
      SELECT b.at_pt FROM belief b JOIN point p ON p.id = b.at_pt
       WHERE b.work_id = w AND b.verb = 'doubts' AND b.entity_id = bl.entity_id AND b.at_pt IS NOT NULL
         AND lint_canon(w, b.prop_id) = lint_canon(w, bl.prop_id) AND precedes(w, bl.from_pt, b.at_pt) = 'yes'
       ORDER BY p.earliest, b.src_id LIMIT 1)
    FROM act s
   WHERE bl.work_id = w AND bl.value AND NOT bl.declared AND s.id = bl.cause_act
     AND ((assertive(s.verb) AND s.verb <> 'shows' AND s.resolved_actor_id IS DISTINCT FROM bl.entity_id)
          OR s.verb = 'reads');

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
    FROM act_content c WHERE c.work_id = w AND c.verb = 'lies' AND c.actor_id IS NOT NULL
  UNION ALL
  SELECT w, a.id, a.resolved_actor_id, p
    FROM act a JOIN artifact art ON art.id = ANY (a.objects) CROSS JOIN LATERAL unnest(art.contains) p
   WHERE a.work_id = w AND a.verb = 'writes' AND a.resolved_actor_id IS NOT NULL
     AND lint_knows_false(w, a.resolved_actor_id, p, a.pt);
END $$;
