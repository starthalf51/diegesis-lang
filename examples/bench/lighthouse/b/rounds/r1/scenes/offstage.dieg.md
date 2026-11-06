---
module: beacon.scenes
import: [beacon.cast]
mode: authored
---

# 무대 밖 사건

```sl
event ev_hide "상자를 숨기다"
  when before @jeongwoo.died by 1d..10d
  jeongseok does hide with boxes_hidden
  gwangho does unload with smuggling

event ev_lastpage "일지의 마지막 쪽"
  when before @jeongwoo.died by 20m..20m
  lasts 5m..10m
  jeongwoo writes logbook
  attr jeongwoo.last_entry = "18시 20분"

event ev_fall "전망대 난간에서"
  when before s1 by 12h..13h
  jeongwoo asks jeongseok: boxes_hidden "기름통 뒤에 숨긴 상자, 네 거냐."
  jeongseok does push jeongwoo with pushed
  dies jeongwoo
  attr jeongwoo.fell_time = "18시 40분"
  jeongseok believes pushed
  jeongseok believes father_lost
  jeongseok does pull_fuse
  jeongseok believes radio_works
  jeongseok does keep_dark with dark_signal

event ev_watch "창가에서 지샌 밤"
  when after @jeongwoo.died by 0m..30m
  lasts 10h..12h
  sunrye sees light_dark
  jeongseok believes light_dark
  attr lamp_room.light_night1 = "돌지 않음"

event ev_call "해경을 부르다"
  when during s6
  mikyung tells jaehee: smuggling
  mikyung tells jaehee: boxes_hidden

event ev_relight "다시 켠 불"
  when after s6 by 10m..1h
  jeongseok does relight with light_turned
  sunrye sees light_turned
  attr lamp_room.light_night3 = "돌았음"

event ev_catch "새벽 항구"
  when after s6 by 2h..7h
  jaehee does arrest gwangho
  jaehee sees gwangho_caught
```
