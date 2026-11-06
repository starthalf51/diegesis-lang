---
module: othello.cyprus
import: [othello.cast]
mode: authored
---

# 키프로스의 무대 밖 사건

```sl
# 3.1.47 에밀리아 "The general and his wife are talking of it; / And she speaks for you stoutly"
event talk "Othello tells Desdemona of Cassio's dismissal"
  when after 2.3
  when before 3.3
  othello tells desdemona: cassio_cashiered src 3.1.47

# 3.3.360 "I will in Cassio's lodging lose this napkin" → 3.4.209 "I found it in my chamber"
event plant "Iago leaves the handkerchief in Cassio's lodging"
  when after 3.3
  when before 3.4
  iago does drop src 3.3.360
  cassio sees: cassio_has_napkin src 3.4.209

# 3.4.190 비앙카가 마지막으로 카시오를 본 때. 카시오가 키프로스에 온(2.1) 뒤다
event last_visit "Cassio's last visit to Bianca"
  when after 2.1
```
