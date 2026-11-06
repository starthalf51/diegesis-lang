---
module: thirdjar.cast
mode: authored
---

# 셋째 항아리: 등장인물과 선언

할머니 최순례의 사십구재 전날, 시골집에 모인 세 남매와 큰고모 정숙. 장독대 셋째 항아리에 든 편지 한 통.

```sl
work thirdjar "셋째 항아리"
  unit day
  start 2026-09-13
  name ko "셋째 항아리"

character daon "김다온" major
  name ko "김다온"
  believes grandma_died
character dohyun "김도현" major
  name ko "김도현"
  believes grandma_died
  believes sell_house
character sua "김수아" major
  name ko "김수아"
  believes grandma_died
character jeongsuk "김정숙" major
  name ko "김정숙"
  believes grandma_died
  believes jeongsuk_absent
  believes ring_taken
  believes daon_birth
  believes raised_by_jeongho
  believes not ring_entrusted
character malsun "윤말순" minor
  name ko "윤말순"
  believes grandma_died
character haewon "혜원 스님" minor
  name ko "혜원 스님"
  believes grandma_died
character sunrye "최순례" minor
  name ko "최순례"
  believes jeongsuk_absent
  believes letter_date
  believes ring_for_daon
  believes open_together
  believes daon_birth
  believes raised_by_jeongho
  believes jar_hint
  believes grandma_wrote

attr daon.age = 29
attr dohyun.age = 41
attr sua.age = 36
attr jeongsuk.age = 63
attr malsun.age = 78
attr sunrye.age = 86
attr daon.relation = "세 남매의 막내"
attr dohyun.relation = "세 남매의 맏이"
attr sua.relation = "세 남매의 둘째"
attr jeongsuk.relation = "세 남매의 큰고모, 순례의 맏딸"
attr malsun.relation = "순례의 옆집 이웃"

place yard "마당"
  name ko "시골집 마당"
place kitchen "부엌"
  name ko "시골집 부엌"
place jangdok "장독대"
  name ko "뒤꼍 장독대"
place anbang "안방"
  name ko "시골집 안방"
place maru "대청마루"
  name ko "시골집 대청마루"
place temple "청련사 법당"
  name ko "청련사 법당"

thing ring "옥가락지"
  name ko "할머니의 옥가락지"
thing jar "셋째 항아리"
  name ko "장독대 셋째 항아리"

attr ring.material = "옥"
attr jar.at = jangdok
attr jar.order = "왼쪽에서 셋째"

fact grandma_died "할머니 최순례는 구월 열나흘 새벽에 돌아가셨다" true
fact ring_entrusted "할머니는 돌아가시기 전날 큰고모 정숙에게 옥가락지를 맡겼다" false
fact jeongsuk_absent "할머니가 돌아가시기 전날인 구월 열사흘, 정숙은 서울에 있었다" true
fact letter_date "셋째 항아리에 든 편지의 날짜는 구월 열사흘이다" true
fact ring_for_daon "할머니는 옥가락지를 다온에게 물려주려 했다" true
fact open_together "속봉투는 정숙이 다온 앞에서 열어야 한다" true
fact daon_birth "다온은 정숙이 낳은 딸이다" true
fact raised_by_jeongho "다온은 정숙의 동생 정호 부부의 딸로 호적에 올라 자랐다" true
fact ring_taken "정숙은 할머니가 돌아가신 다음 날 장롱 서랍에서 옥가락지를 꺼내 갔다" true
fact jar_hint "할머니는 셋째 항아리를 사십구재 전날에 열어 보라고 일렀다" true
fact grandma_wrote "할머니는 돌아가시기 전날 마루에 앉아 편지를 쓰고 있었다" true
fact sell_house "도현은 사십구재가 끝나면 시골집을 내놓을 생각이다" true

artifact grandma_letter "할머니의 편지"
  name ko "할머니의 편지"
  contains letter_date, jeongsuk_absent, ring_for_daon, open_together
  "구월 열사흘. 오늘 정숙이는 서울 가고 집에 나 혼자다."
artifact inner_letter "속봉투"
  name ko "정숙에게 쓴 속봉투"
  contains daon_birth, raised_by_jeongho
  "정숙아, 이제는 네 입으로 말해 주어라."
```
