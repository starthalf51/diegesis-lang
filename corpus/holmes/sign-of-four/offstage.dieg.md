---
module: holmes.four
import: [holmes.cast]
mode: authored
---

# 『네 사람의 서명』의 무대 밖 사건

```sl
# 2.17 (f1 = 1888-07-07, 2.19-2.20) "About six years ago—to be exact, upon the 4th of May, 1882 … The same day there arrived through the post a small
# card-board box addressed to me, which I found to contain a very large and lustrous pearl."
event first_pearl "The first pearl comes to Mary Morstan"
  when before f1 by 6y..7y
  mary sees: pearls_sent src 2.17
```
