---
module: macbeth.act2
import: [macbeth.cast]
mode: authored
---

# 2막의 무대 밖 사건 — 살해는 무대에 나오지 않는다

```sl
event murder "Macbeth murders Duncan in his chamber"
  when after 2.1
  # 2.2.5 "He is about it": 살해는 2.2 첫머리와 겹친다. 맥베스가 들어오기(deed_done) 전에 끝난다
  when before 2.2.deed_done
  macbeth does murder duncan src 2.2.18
  dies duncan
  macbeth sees: not alive(duncan) src 2.2.18
```
