---
module: yeoulseom.cast
mode: authored
---

# 여울섬 — 3화 추가 선언

```sl
place beach "tidal flat beach"
  name ko "여울섬 갯가"

character taeil "Seo Taeil" minor
  name ko "서태일"
  believes vote_in_12
  believes official_cause

fact minjae_leaves "강민재는 육지 회사 일로 급히 섬을 떠나 돌아가야 한다" true

minjae believes minjae_leaves
```

```sl
attr taeil.age = 34
```
