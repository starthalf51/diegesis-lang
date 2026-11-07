---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 6화 추가 선언

강민재는 3화에 육지로 떠나 퇴장했고 다시 나오지 않는다. 서윤을 돕는 자리는 분교 교사 윤유진이 맡는다.

장부는 등대에 머무는 물건(thing)이다. 서윤이 읽는 것은 그 안에 적힌 근무 기록(artifact)이다.

```sl
thing lighthouse_logbook "lighthouse duty logbook"
  name ko "등대 근무 장부"

fact logbook_kept_daily "등대 근무 장부는 하루도 빠짐없이 날마다 적혀 있다" true
fact logbook_day_torn "등대 근무 장부에서 해진호 사고 당일 쪽만 찢겨 나가 있다" true
fact tell_only_yujin "서윤은 장부 일을 유진에게만 털어놓기로 한다" true
fact boxes_moving "류지환이 마을회관 창고의 기록물 상자를 선착장으로 옮길 채비를 하고 있다" true
fact keep_secret "유진은 장부 일을 누구에게도 말하지 않겠다고 약속한다" true

artifact logbook_entries "lighthouse duty log entries"
  name ko "등대 근무 장부의 기록"
  contains logbook_kept_daily
  "날마다 같은 손글씨로 적힌 근무 기록"

jihwan believes boxes_moving
yujin believes keep_secret
```

```sl
attr lighthouse_logbook.at = lighthouse
```
