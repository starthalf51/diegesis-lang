---
module: yeoul.cast
mode: authored
---

# 여울섬 — 2화 추가 선언

```sl
fact memorial_thirty "오늘 선착장에서 해진호 삼십 주기 위령제가 열린다" true
fact chrysanthemum_rite "유족들이 바다에 국화를 한 송이씩 던진다" true
fact gyeongtae_hosts "도경태는 이장으로서 해마다 해진호 위령제를 주관해 왔다" true
fact appendix_story "사고 사흘 전 도경태는 맹장이 터져 육지로 실려 갔다" false
fact box_off_limits "장롱 위 양철 상자는 오말순이 아무도 손대지 못하게 하는 물건이다" true
fact sank_eleventh_tide "해진호가 가라앉은 밤은 열한 물때였다" true

gyeongtae believes memorial_thirty
gyeongtae believes gyeongtae_hosts
gyeongtae believes not appendix_story
malsun believes box_off_limits
malsun believes sank_eleventh_tide
malsun believes haejin_sank
```
