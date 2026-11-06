---
module: gangdo.scenes
import: [gangdo.cast]
mode: authored
---

```sl
event note_written "새벽, 강철수가 부엌 등불 아래에서 쪽지를 쓴다"
  when before s1 by 10m..1h
  lasts 5m..10m
  chulsu writes note
```
