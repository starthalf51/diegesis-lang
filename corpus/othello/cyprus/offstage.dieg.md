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

# 3.4.190 비앙카가 마지막으로 카시오를 본 때. 카시오가 키프로스에 온(2.1) 뒤다 — 전제(해석): 두 사람의 왕래는 섬에서 이뤄진다.
# 근거: 3.4.188-189 "I was coming to your house." / "And I was going to your lodging, Cassio."(서로의 숙소를 오간다),
# 4.1.149-152 "she haunts me in every place. I was the other day talking on the sea-bank with certain Venetians; and thither comes
# the bauble"(카시오가 섬에 있는 동안 그녀가 쫓아다닌다). 마지막 방문의 날짜는 원문에 없다.
event last_visit "Cassio's last visit to Bianca"
  when after 2.1
```
