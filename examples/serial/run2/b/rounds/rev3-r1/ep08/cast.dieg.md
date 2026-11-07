---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 8화 추가 선언

강민재는 3화에 육지로 떠나 퇴장했고 다시 나오지 않는다. 서윤을 돕는 자리는 분교 교사 윤유진이 맡는다.

문기철은 해진호가 가라앉은 밤 선착장에서 도경태를 직접 봤다. 비옷 차림의 도경태는 폭풍 경보에도 망설이는 선장을 다그쳐 배를 띄우게 했다. 그는 이장이 퍼뜨린 입원 이야기를 따라 말해 왔지만, 그 밤의 기억은 삼십 년 동안 그대로 갖고 있었다.

```sl
fact gicheol_saw_gt "해진호가 가라앉은 밤 문기철은 선착장에서 비옷을 입은 도경태가 망설이는 선장을 다그쳐 배를 띄우게 하는 것을 봤다" true
fact gicheol_silent "문기철은 민박 허가를 이장 도경태에게 빚진 탓에 삼십 년 동안 입을 다물었다" true
fact confront_plan "서윤은 투표 전에 이장 도경태를 직접 찾아가 그 밤의 일을 묻기로 한다" true

gicheol believes gicheol_saw_gt
gicheol believes gicheol_silent
gicheol believes forced_sailing
```
