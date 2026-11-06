---
module: othello.venice
import: [othello.cast]
mode: authored
---

```sl
# 1.1-1.2: 데스데모나가 그 밤에 아버지 집을 나와 오셀로와 결혼한다(1.2.58 "to-night hath boarded a land carack")
event wedding "Othello marries Desdemona"
  lasts 0d..6h
  othello sees: married src 1.2.58
  desdemona sees: married src 1.2.58
  iago sees: married src 1.1.125                # 이아고는 도주를 지켜보고 브라반쇼에게 알린다
```
