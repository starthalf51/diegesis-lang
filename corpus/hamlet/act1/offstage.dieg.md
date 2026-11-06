---
module: hamlet.act1
import: [hamlet.cast]
mode: authored
---

# 1막의 무대 밖 사건

```sl
# 1.5.59-73 선왕은 정원에서 잠들었다가 동생이 귀에 부은 독으로 죽었다
event murder "Claudius poisons King Hamlet in the orchard"
  claudius does poison old_hamlet src 1.5.66
  dies old_hamlet

# 1.1: 호레이쇼가 마셀러스·버나도와 망대에서 유령을 본다. 1.2.192 "I think I saw him yesternight"
event first_sighting "the Ghost appears to Horatio on the platform"
  when after murder
  when before 1.2 by 0d..1d
  horatio sees: ghost_walks src 1.1.49
  marcellus sees: ghost_walks src 1.1.49
  bernardo sees: ghost_walks src 1.1.49
```
