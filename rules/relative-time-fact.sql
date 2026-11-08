-- rule: relative-time-fact
-- severity: warn
-- layer: lint
-- target: fact
-- summary: 사실 텍스트에 상대 시간("열이틀 뒤", "in twelve days")이 박혀 있다
--
-- 사실은 장면이 바뀌어도 같은 말이어야 한다. 기준점이 없는 "N일 뒤"는 고치다 어긋난 흔적이다(절대 날짜나 `when` 으로 쓴다).
-- 대상: fact.text(별칭 사실의 text 는 이름이라 걸리지 않는다). 한국어 수사+단위+뒤/후/전에/만에, 영어 in|after N days|weeks|months|years.
SELECT 'fact' AS target_kind, f.id AS target_id, f.src_id,
       format('fact %s names a relative time: %s', lint_short(f.id), f.text) AS message,
       '{}'::jsonb AS data
  FROM fact f
 WHERE f.work_id = $1
   AND (f.text ~ '([0-9]+|[한두세네다섯여섯일곱여덟아홉열스무]+|열[한두세네]?|이틀|사흘|나흘|닷새|엿새|이레|여드레|아흐레|열흘|보름)\s*(일|날|주|달|개월|년|시간)?\s*(뒤|후|전에|만에)'
     OR f.text ~* '\y(in|after)\s+\w+\s+(days?|weeks?|months?|years?)\y')
