---
module: holmes.scarlet
import: [holmes.cast]
mode: authored
---

# 『주홍색 연구』의 무대 밖 사건

```sl
# 1.1.1-2: 1878 년 학위, 제2차 아프간 전쟁, 마이완드 전투에서 Jezail 총알을 맞는다. 당번병 머리가 그를 구해 낸다
event afghan_campaign "Watson's service in the second Afghan war"
  watson sees: watson_wounded src 1.1.2
  murray sees: watson_wounded src 1.1.2
```

```sl
# 1.1.12 스탬퍼드: "A fellow who is working at the chemical laboratory up at the hospital. He was bemoaning himself this
# morning because he could not get someone to go halves with him in some nice rooms which he had found" — 같은 날 아침
event holmes_complains "Holmes complains to Stamford about the rooms"
  when before s1 by 0d..12h
  holmes tells stamford: holmes_seeks_partner src 1.1.12
```
