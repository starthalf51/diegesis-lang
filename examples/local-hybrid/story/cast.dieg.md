---
module: sea_regressor.cast
mode: authored
---

```sl
work sea_regressor "The Keeper's Debt"
  unit day
  name ko "등대지기의 부채"

character haneol "한얼" major
  name ko "한얼"
  believes haneol_again
  believes letter_hand
  believes letter_claim
  believes letter_order
  believes wreck_ten
  believes light_was_out

character jiwoo "지우" major
  name ko "지우"
  believes wreck_ten

character minho "민호" minor
  name ko "민호"
  believes wreck_ten

character sookhee "숙희" minor
  name ko "숙희"
  believes wreck_ten

character kim "김 형사" minor
  name ko "김 형사"

character seol "설이" extra
  name ko "설이"

place lighthouse "등대"
  name ko "외딴 섬의 등대"

place village "village"
  name ko "어촌 마을"

place pier "pier"
  name ko "낡은 부두"

place old_house "old house"
  name ko "숙희의 집"

place station "station"
  name ko "파출소"

thing light "등불"
  name ko "등불"

thing sea "바다"
  name ko "바다"

thing sky "하늘"
  name ko "하늘"

thing boat "작은 배"
  name ko "작은 배"

thing photo "낡은 사진"
  name ko "낡은 사진"

thing file "사건 서류"
  name ko "사건 서류"

artifact letter "낡은 편지"
  name ko "낡은 편지"
  contains letter_claim, letter_order
  "오늘 밤 등을 끄고 배를 띄워라"

fact wreck_ten "십 년 전 폭풍 밤 영진호가 암초에 부딪혀 여섯 명이 죽었다" true
fact light_was_out "그 밤 등대의 불은 꺼져 있었다" true
fact letter_hand "편지의 글씨는 한얼 자신의 것이다" true
fact haneol_again "한얼은 이 밤을 이미 한 번 살았다" true
fact storm_tonight "오늘 밤 폭풍이 온다" true
fact letter_claim "바다가 오늘 밤 빚을 받으러 온다" true
fact letter_order "등을 끄고 배를 띄워라" true
fact no_letter "한얼은 편지를 받은 적이 없다" false
fact minho_home "민호는 어젯밤 집에 있었다" false
fact minho_took_letter "민호가 등대에서 편지를 가져갔다" true
fact boat_out "민호가 배를 띄웠다" true

attr light.at = lighthouse
attr photo.at = old_house
```
