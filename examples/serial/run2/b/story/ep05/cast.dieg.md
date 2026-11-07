---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 5화 추가 선언

강민재는 3화에서 이미 육지로 떠나 이야기에서 퇴장했다. 분교에 새로 온 교사 윤유진이 서윤을 돕는 자리를 맡는다.

```sl
character yujin "Yoon Yujin" minor
  name ko "윤유진"
  believes yujin_teacher

thing nail_box "box of new nails"
  name ko "새 못이 든 상자"
thing island_records "island archive boxes"
  name ko "섬 기록물 상자"

fact yujin_teacher "윤유진은 여울섬 분교에 새로 부임한 교사다" true
fact youngho_lives "백영호는 섬 끝 폐가에서 몰래 지내고 있다" true
fact nails_match "폐가 상자 속 새 못은 등대 열세 번째 계단의 새 못과 똑같다" true
fact step_by_youngho "등대 열세 번째 계단 칸을 최근에 뜯었다가 다시 박은 사람은 백영호다" true
fact archive_boxes "마을회관 창고에는 해진호 사고 무렵의 섬 기록물 상자들이 쌓여 있다" true
fact archive_out "류지환은 투표 전에 섬 기록물을 개발사 창고로 빼돌리려 한다" true
fact records_moldy "섬 기록물이 습기에 상해 육지 보관소로 보내야 한다" false
fact record_1993 "기록물 상자 하나에 해진호 사고 날 밤의 섬 출입 기록이 들어 있다" true

youngho believes youngho_lives
youngho believes step_by_youngho
youngho believes nails_match
jihwan believes archive_boxes
jihwan believes archive_out
jihwan believes record_1993
```

```sl
attr yujin.age = 27
attr nail_box.at = ruin_house
```
