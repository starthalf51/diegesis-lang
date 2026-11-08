---
module: yeoul.cast
mode: authored
---

# 여울섬 — 5화 무대 밖 사건

```sl
event taeo_departs "태오가 아침 첫 배로 섬을 떠난다"
  when after s11 by 0d..1d
  lasts 30m..60m
  taeo believes taeo_recalled
  taeo tells haesol: taeo_recalled
  attr taeo.status = gone
```
