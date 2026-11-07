---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 7화 추가 선언

강민재는 3화에 육지로 떠나 퇴장했고 다시 나오지 않는다. 서윤을 돕는 자리는 분교 교사 윤유진이 맡는다.

입원 서류는 도경태의 집 서랍에 머무는 물건이다. 하은은 서류를 보고 제자리에 돌려놓는다.

```sl
place gyeongtae_house "Gyeongtae's house"
  name ko "도경태의 집"

thing admission_papers "hospital admission papers"
  name ko "도경태의 입원 서류"

fact admit_date_off "도경태의 입원 서류에 적힌 입원 날짜는 해진호 사고 다음 날이다" true
fact papers_in_envelope "입원 서류는 아버지 서랍 맨 아래 누런 봉투 속에 접혀 있다" true
```

```sl
attr admission_papers.at = gyeongtae_house
```
