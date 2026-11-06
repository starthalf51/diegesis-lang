---
module: chibi.cast
mode: authored
---

# 적벽 — 등장인물과 선언

원문: `texts/brewitt-taylor-45-47.txt` (C. H. Brewitt-Taylor 역 『San Kuo, or Romance of the Three Kingdoms』 1925,
Project Gutenberg #77416). `src` 는 `<회>.<단락>` 이다(회 제목이 단락 1). 정사 대조는 `texts/sanguozhi.txt`
(陳壽 『三國志』 周瑜傳·武帝紀, 중국어 위키문헌). 영어 이름은 Brewitt-Taylor 의 웨이드식 표기다.

포팅 범위: 45회 후반(주유의 정찰 45.77 ~ 공명이 계책을 알아챔 46.10), 46회 후반(채중·채화의 거짓 투항 46.67 ~
감택이 서신을 맡음 46.127), 47회 전반(감택의 서신 전달 47.6 ~ 장간의 재파견 47.65). 화살 빌리기(46.16-66)와
방통의 연환계(47.66 이후)는 뺐다.

```sl
work chibi "San Kuo, or Romance of the Three Kingdoms — the ruses before Red Cliffs"
  unit day
  name ko "삼국지연의 — 적벽 전야"
  name en "Romance of the Three Kingdoms — before Red Cliffs"
  name zh "三國演義 — 赤壁"

character cao_cao "Cao Cao", "Tsʻao Tsʻao" major
  name ko "조조"
  name en "Tsʻao Tsʻao"
  name zh "曹操"
character zhou_yu "Zhou Yu", "Chou Yü" major
  name ko "주유"
  name en "Chou Yü"
  name zh "周瑜"
character jiang_gan "Jiang Gan", "Chiang Kan" major
  name ko "장간"
  name en "Chiang Kan"
  name zh "蔣幹"
character cai_mao "Cai Mao", "Tsʻai Mao" minor
  name ko "채모"
  name en "Tsʻai Mao"
  name zh "蔡瑁"
character zhang_yun "Zhang Yun", "Chang Yün" minor
  name ko "장윤"
  name en "Chang Yün"
  name zh "張允"
character huang_gai "Huang Gai", "Huang Kai" major
  name ko "황개"
  name en "Huang Kai"
  name zh "黃蓋"
  believes huang_gai_loyal          # 46.89 "The Sun family have been very generous to me"
character kan_ze "Kan Ze", "Kʻan Tsê" major
  name ko "감택"
  name en "Kʻan Tsê"
  name zh "闞澤"
character zhuge_liang "Zhuge Liang", "Kʻung-ming" major
  name ko "제갈량"
  name en "Kʻung-ming"
  name zh "諸葛亮"
character lu_su "Lu Su" minor
  name ko "노숙"
  name en "Lu Su"
  name zh "魯肅"
character gan_ning "Gan Ning", "Kan Ning" minor
  name ko "감녕"
  name en "Kan Ning"
  name zh "甘寧"
character cai_zhong "Cai Zhong", "Tsʻai Chung" minor
  name ko "채중"
  name en "Tsʻai Chung"
  name zh "蔡中"
character cai_he "Cai He", "Tsʻai Ho" minor
  name ko "채화"
  name en "Tsʻai Ho"
  name zh "蔡和"
character taishi_ci "Taishi Ci", "Tʻaishih Tzŭ" extra
  name ko "태사자"
  name en "Tʻaishih Tzŭ"
  name zh "太史慈"
group wu_officers "the officers of Wu" extra
  name ko "오의 장수들"
  name en "the officers of Wu"
  believes commanders_loyal         # 45.77 "they told him Tsʻai Mao and Chang Yün": 조조 수군 도독의 이름을 안다
group cao_officers "Cao's officers" extra
  name ko "조조의 장수들"
  name en "Tsʻao's officers"
character wu_spies "spies of Wu" extra       # 45.140 "Spies took the news to Chou Yü"
  name ko "오의 첩자"
  name en "spies"
character lictors "the lictors" extra
  name ko "형리"
  name en "the lictors"

# 기록의 출처(account … by). 서술자는 사건을 전하는 쪽이라 정보 흐름 검사에서 뺀다
thing romance "Romance of the Three Kingdoms"
  name ko "삼국지연의"
  name zh "三國演義"
thing sanguozhi "Records of the Three Kingdoms"
  name ko "정사 삼국지"
  name zh "三國志"
character luo_guanzhong "Luo Guanzhong" extra
  name ko "나관중"
  name zh "羅貫中"
  knows omniscient
character chen_shou "Chen Shou" extra
  name ko "진수"
  name zh "陳壽"
  knows omniscient
character pei_songzhi "Pei Songzhi" extra   # 裴松之 注(《江表傳》 인용)
  name ko "배송지"
  name zh "裴松之"
  knows omniscient

place cao_camp "Tsʻao's camp on the north bank"
  name ko "강북의 조조 진영"
  name en "Tsʻao's camp"
place zhou_camp "Chou Yü's camp on the south bank"
  name ko "강남의 주유 진영"
  name en "Chou Yü's camp"
place river "the Great River"
  name ko "장강"
  name en "the River"
place liang_boat "Kʻung-ming's little craft"
  name ko "공명의 작은 배"
  name en "Kʻung-ming's boat"
place gan_ning_camp "Kan Ning's camp"
  name ko "감녕의 진영"
  name en "Kan Ning's camp"

role chancellor "the Prime Minister"
  name ko "승상"
  name en "the Prime Minister"
  name zh "丞相"
  held cao_cao from start               # 45.84 "Rest contented, O Minister"

fact commanders_traitors "Tsʻai Mao and Chang Yün secretly serve Wu" false
fact commanders_loyal = not commanders_traitors   # 45.77-78 조조의 수군 도독이고 수전에 능하다
fact letter_forged "the letter from Tsʻai and Chang was forged by Chou Yü" true
fact cao_head_soon "Tsʻao's head will be taken within days" false
fact jiang_gan_envoy "Chiang Kan comes to persuade Chou Yü for Tsʻao" true
fact not_envoy = not jiang_gan_envoy
fact commanders_remiss "the two admirals were executed for negligence" false
fact commanders_executed "Tsʻai Mao and Chang Yün are put to death" true
fact cai_brothers_defect "Tsʻai Chung and Tsʻai Ho truly desert to Wu" false
fact cai_brothers_spies = not cai_brothers_defect
fact huang_gai_defects "Huang Kai means to desert to Tsʻao" false
fact huang_gai_loyal = not huang_gai_defects
fact huang_gai_beaten "Huang Kai was beaten before all the officers" true
fact beating_is_ruse "the beating is a ruse to make the desertion credible" true
fact gan_ning_defects "Kan Ning means to betray Chou Yü" false
# 정사 대조용
fact jiang_gan_visits "Chiang Kan visits Chou Yü to win him over" true
fact letter_stolen "Chiang Kan steals a letter from Chou Yü's tent" true
fact huang_gai_feigns_surrender "Huang Kai sends Tsʻao a letter feigning surrender" true
fact plague_in_cao_army "sickness is already in Tsʻao's army" true

artifact forged_letter "the letter marked as from Chang Yün and Tsʻai Mao"
  contains commanders_traitors
  "We surrendered to Tsʻao Tsʻao, not for the sake of pay but under stress of circumstances."
artifact surrender_letter "Huang Kai's letter"
  contains huang_gai_defects
  "I, Huang Kai, have been generously treated by the Sun family"
artifact spy_report "the letter of the two Tsʻais on the beating"
  contains huang_gai_beaten
artifact kan_letter "Kʻan Tsê's second letter"
  contains huang_gai_defects
  "when he came his boat could be recognised by a black, indented flag"
artifact cai_letter "the two Tsʻais' letter on Kan Ning"
  contains gan_ning_defects
```
