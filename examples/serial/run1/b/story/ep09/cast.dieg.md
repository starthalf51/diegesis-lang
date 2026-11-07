---
module: haejin.cast
mode: authored
---

# 여울섬 — 9화 추가 선언

```sl
fact youngho_climbs "계단 열세 번째 칸에 새 못을 박고 밤마다 버려진 등대를 오르내린 사람은 백영호다" true
fact youngho_wants_light "백영호는 등대가 헐리기 전에 꼭대기에 한 번 더 올라 그날 밤을 마주하고 싶었다" true
fact light_went_out_eleven "그날 밤 등대 불은 밤 열한 시 무렵에 꺼졌다" true
fact light_back_one "등대 불은 새벽 한 시 무렵에야 다시 켜졌다" true
# 백영호의 반쪽 고백 — 술에 취해 잠든 부끄러움을 감추려고 지어낸 말이다(youngho_fell_asleep 과 반대)
fact youngho_off_by_order "백영호는 도경태의 지시를 받고 등대 불을 두 시간 동안 일부러 껐다" false
fact youngho_shaking "고백하는 백영호의 손이 술잔을 쥔 채 떨린다" true
fact seoyun_shaken "서윤은 삼십 년 동안 들어 온 이야기가 무너지는 것을 느낀다" true
fact eleventh_tide_eleven_pm "노래의 '열한 물때에 불이 잔다'는 해진호가 가라앉은 밤 열한 시에 등대 불이 꺼졌다는 뜻이다" true

youngho believes youngho_climbs
youngho believes youngho_wants_light
youngho believes light_went_out_eleven
youngho believes light_back_one
youngho believes youngho_shaking
seoyun believes seoyun_shaken
```
