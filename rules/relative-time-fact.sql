-- rule: relative-time-fact
-- severity: warn
-- layer: lint
-- target: fact
-- summary: a fact text has a relative time embedded ("twelve days later" in Korean, "in twelve days")
--
-- A fact must read the same even when the scene changes. An "N days later" with no reference point is a trace of editing gone wrong (use an absolute date or `when`).
-- Target: fact.text (an alias fact's text is a name, so it is not caught). Korean numeral+unit+later/after/before/within, English in|after N days|weeks|months|years.
-- What matches without a unit is only words that are a duration by themselves (two days, three days, a half month …). "after confirming" and "after that" are not numerals, so a unit is required.
SELECT 'fact' AS target_kind, f.id AS target_id, f.src_id,
       format('fact %s names a relative time: %s', lint_short(f.id), f.text) AS message,
       '{}'::jsonb AS data
  FROM fact f
 WHERE f.work_id = $1
   AND (f.text ~ '([0-9]+|[한두세네다섯여섯일곱여덟아홉열스무]+)\s*(일|날|주|달|개월|년|시간)\s*(뒤|후|전에|만에)'
     OR f.text ~ '(이틀|사흘|나흘|닷새|엿새|이레|여드레|아흐레|열흘|보름)\s*(뒤|후|전에|만에)'
     OR f.text ~* '\y(in|after)\s+\w+\s+(days?|weeks?|months?|years?)\y')
