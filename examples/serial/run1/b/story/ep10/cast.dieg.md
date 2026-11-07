---
module: haejin.cast
mode: authored
---

# 여울섬 — 10화 추가 선언

```sl
fact ledger_torn "그날 밤 등대 불이 꺼진 일은 등대 근무 장부에 적혔지만 그 쪽은 찢겨 사라졌다" true
# 서윤의 짐작 — 실제로 장부 쪽은 오말순이 갖고 있다(malsun_keeps_page)
fact page_under_nail "찢긴 장부 쪽은 등대 계단 열세 번째 칸의 새 못 아래에 숨겨져 있다" false
fact empty_groove "새 못을 뽑아낸 열세 번째 디딤판 밑에는 손바닥만 한 홈이 파여 있지만 그 안은 텅 비어 있다" true
fact page_elsewhere "찢긴 장부 쪽은 등대가 아닌 다른 곳에 있다" true
fact malsun_grips_box "장부 이야기가 나오자 오말순은 반짇고리를 무릎으로 끌어당겨 두 손으로 감싼다" true

youngho believes ledger_torn
malsun believes ledger_torn
```
