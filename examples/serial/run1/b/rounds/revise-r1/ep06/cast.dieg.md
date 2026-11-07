---
module: haejin.cast
mode: authored
---

# 여울섬 — 6화 추가 선언

```sl
place mainland_hospital "the mainland hospital"
  name ko "육지 병원"

character clerk "the night records clerk" extra
  name ko "육지 병원 원무과 야간 직원"
  believes kyungtae_no_record
  believes log_complete

artifact admission_log "the old admission register"
  name ko "삼십 년 전 입원 대장"
  contains kyungtae_no_record, log_complete

fact youngho_scoffed "서윤이 이장은 그날 밤 육지 병원에 있었다고 하자 백영호는 코웃음을 쳤다" true
fact nail_bag "폐가 툇마루에 녹슬지 않은 새 못이 든 봉지가 놓여 있다" true
fact stay_away "서윤은 다시는 섬 끝 폐가에 찾아오지 말아야 한다" true
fact minjae_will_check "민재는 육지 병원에 가서 사고 날 밤의 입원 기록을 직접 확인해 보겠다고 한다" true
fact kyungtae_no_record "삼십 년 전 해진호가 가라앉은 밤의 육지 병원 입원 대장에는 도경태의 이름이 없다" true
fact log_complete "그 무렵의 입원 대장은 한 장도 빠지지 않고 남아 있다" true

youngho believes stay_away
minjae believes minjae_will_check
```
