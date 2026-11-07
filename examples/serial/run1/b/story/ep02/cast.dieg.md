---
module: haejin.cast
mode: authored
---

# 여울섬 — 2화 추가 선언

```sl
fact memorial_day "며칠 뒤면 해진호 사고로 돌아가신 서윤 아버지의 기일이다" true
fact seoyun_will_ask "서윤은 아버지 기일을 앞두고 이장에게 사고 날 밤의 일을 직접 묻기로 한다" true
fact kyungtae_hand_paused "서윤이 아버지 이야기를 꺼내자 도경태는 담배를 쥔 손을 잠시 멈췄다" true
fact new_nail "버려진 등대 계단 열세 번째 칸에 녹슬지 않은 새 못이 박혀 있다" true
fact minjae_leaves "민재는 급한 회사 일로 오늘 오후 배를 타고 육지로 떠나 섬에 돌아오지 않는다" true
fact taeo_nurse "윤태오는 여울섬 보건지소에서 일하며 오말순을 돌보고 있다" true

character taeo "Yoon Taeo" major
  name ko "윤태오"
  attr taeo.age = 30
  believes taeo_nurse
  believes grandma_memory_failing

seoyun believes memorial_day
minjae believes minjae_leaves
```
