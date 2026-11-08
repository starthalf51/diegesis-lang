---
module: yeoul.cast
mode: authored
---

# 여울섬 — 5화 무대 밖 사건

```sl
event taeo_departs "태오가 선착장에서 해솔에게 떠난다는 말을 남긴다"
  when after s32 by 0d..1d
  lasts 10m..30m
  taeo believes taeo_recalled
  taeo tells haesol: taeo_recalled

event taeo_gone "태오를 태운 첫 배가 섬을 떠난다"
  when after taeo_departs.end by 0m..30m
  lasts 5m..10m
  attr taeo.status = gone
```
