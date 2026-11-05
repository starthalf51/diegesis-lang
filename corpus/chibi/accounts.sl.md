---
module: chibi.accounts
import: [chibi.cast, chibi.ch45, chibi.ch47]
mode: authored
---

# 연의와 정사 — 적벽 전야의 계책에 대한 두 기록

같은 사건(적벽 전야 오의 계책)을 『삼국지연의』(45~47회)와 陳壽 『三國志』(`texts/sanguozhi.txt`)가 어떻게 전하는지
담는다. 정사 쪽은 陳壽 본문(周瑜傳·武帝紀)과 裴松之 注가 인용한 《江表傳》이다.

- 정사에는 蔣幹盜書가 없다. 陳壽 본문에는 蔣幹이 아예 없고, 裴注의 蔣幹은 周瑜를 달래러 왔다가 "雅量高致" 를
  칭찬하며 돌아갈 뿐이다. 채모·장윤의 처형도 없다.
- 정사에는 苦肉計(황개가 매를 맞음)가 없다. 黃蓋는 서신으로 거짓 항복을 알린다("先書報曹公，欺以欲降").
- 정사는 조조 군에 이미 병이 돌았다고 적는다(周瑜傳 "時曹公軍眾已有疾病", 武帝紀 "於是大疫"). 45~47회에는 없다.

```sl
event chibi_ruses "the ruses of Wu before Red Cliffs"
  when after start

account romance_account of chibi_ruses by romance
  luo_guanzhong says: jiang_gan_visits "the messenger reached Chou Yü's camp and bade the men say that an old friend Chiang Kan wished to see him" src 45.88
  luo_guanzhong says: letter_stolen "he secreted the letter in his dress" src 45.116
  luo_guanzhong says: commanders_executed "Tsʻao bade the executioners lead them away to instant death" src 45.136
  luo_guanzhong says: huang_gai_beaten "So Huang Kai was stripped, thrown to the ground and fifty blows were given" src 46.102
  luo_guanzhong says: huang_gai_feigns_surrender "Kʻan Tsê presents the treacherous letter" src 47.1

account sanguozhi_account of chibi_ruses by sanguozhi
  pei_songzhi says: jiang_gan_visits "遣九江蔣幹往見瑜 … 幹還，稱瑜雅量高致，非言辭所間" src 裴注江表傳
  chen_shou says: plague_in_cao_army "時曹公軍眾已有疾病" src 周瑜傳
  chen_shou says: huang_gai_feigns_surrender "先書報曹公，欺以欲降" src 周瑜傳
```
