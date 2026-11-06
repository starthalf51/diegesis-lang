---
module: thirdjar.scenes
import: thirdjar.cast
mode: authored
---

# 무대 밖 사건

```sl
event letter_day "할머니가 편지를 쓴 날"
  when at 2026-09-13 noon
  attr jeongsuk.whereabouts = "서울"
  sunrye writes grandma_letter
  sunrye writes inner_letter
  malsun sees grandma_wrote
  sunrye tells malsun: jar_hint "셋째 항아리는 사십구재 전날에나 열어 보라고 애들한테 꼭 일러 주게."

event grandma_death "할머니가 돌아가신 새벽"
  when at 2026-09-14 dawn
  dies sunrye

event ring_taken_day "정숙이 반지를 꺼낸 날"
  when at 2026-09-15 morning
  jeongsuk does take ring
```
