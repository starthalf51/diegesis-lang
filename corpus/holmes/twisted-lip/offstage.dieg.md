---
module: holmes.lip
import: [holmes.cast]
mode: authored
---

# 「입술이 비뚤어진 사나이」의 무대 밖 사건

```sl
# 6.10 "He has not been home for two days", 6.12 "She had the surest information that of late he had, when the fit was
# on him, made use of an opium den in the farthest east of the City" — 케이트가 왓슨 부부를 찾아오기 전
event kate_learns "Kate Whitney learns where Isa is"
  when before l1 by 0d..2d
  kate sees: isa_missing src 6.10
  kate sees: isa_at_den src 6.12
```
