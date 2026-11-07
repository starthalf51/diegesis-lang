---
module: haejin.cast
mode: authored
---

# 여울섬 — 7화 추가 선언

```sl
place kyungtae_house "the village head's house"
  name ko "이장 도경태의 집"

thing father_desk "Kyungtae's old desk"
  name ko "도경태의 낡은 책상"
  attr father_desk.at = kyungtae_house

artifact ferry_ticket "the old ferry ticket"
  name ko "삼십 년 전 배표"
  contains ticket_date

parent kyungtae of haeun

fact haeun_looking_for "하은은 분교 아이들에게 보여 줄 옛 섬 사진을 찾으러 아버지 책상 서랍을 연다" true
fact ticket_in_drawer "아버지 책상 맨 아래 서랍 안쪽 틈에 누렇게 바랜 배표 한 장이 끼여 있다" true
fact ticket_date "배표에는 해진호가 가라앉은 날 저녁, 육지에서 여울섬으로 들어오는 배의 날짜와 시각이 찍혀 있다" true
fact haeun_cannot_tell "하은은 배표 이야기를 서윤에게 차마 꺼내지 못한다" true
fact kyungtae_never_spoke "아버지는 사고 날 밤 이야기를 집에서 한 번도 꺼낸 적이 없다" true
fact minjae_will_tell "민재는 이 기록을 서윤에게 알려야 한다고 생각한다" true
fact seoyun_will_meet_kyungtae "서윤은 기일 전에 이장을 다시 찾아가 그날 밤 이야기를 들을 생각이다" true

haeun believes haeun_looking_for
haeun believes kyungtae_never_spoke
minjae believes minjae_will_tell
seoyun believes seoyun_will_meet_kyungtae
```
