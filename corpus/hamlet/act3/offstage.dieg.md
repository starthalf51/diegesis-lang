---
module: hamlet.act3
import: [hamlet.cast, hamlet.act1]
mode: authored
---

# 3막의 무대 밖 사건

```sl
# 3.2.78 "Which I have told thee of my father's death": 1.5 와 3.2 사이 어느 때 햄릿이 호레이쇼에게 털어놓았다
event confides "Hamlet tells Horatio what the Ghost said"
  when after 1.5
  when before 3.2
  hamlet tells horatio: claudius_murdered src 3.2.78
```
