---
module: yeoulseom.scenes
import: [yeoulseom.cast]
mode: authored
---

# 여울섬 — 5화 무대 밖 사건

```sl
event minjae_departs "강민재가 첫배로 섬을 떠난다"
  when after s11
  when before s12
  minjae tells seoyun: minjae_leaves
  minjae does depart
  attr minjae.status = gone
```
