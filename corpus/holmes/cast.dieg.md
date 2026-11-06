---
module: holmes.cast
mode: authored
---

# 셜록 홈즈 — 등장인물과 선언

원문(모두 Project Gutenberg, 퍼블릭 도메인, 2026-11-06 수집):

| 모듈 | 작품 | 파일 | URL | `src` |
|---|---|---|---|---|
| `holmes.scarlet` (`study-in-scarlet/`) | A Study in Scarlet (1887) | `texts/study-in-scarlet-pg244.txt` | https://www.gutenberg.org/cache/epub/244/pg244.txt | `<부>.<장>.<단락>` (1부 표제 = 0장) |
| `holmes.four` (`sign-of-four/`) | The Sign of the Four (1890) | `texts/sign-of-four-pg2097.txt` | https://www.gutenberg.org/cache/epub/2097/pg2097.txt | `<장>.<단락>` |
| `holmes.lip` (`twisted-lip/`) | The Man with the Twisted Lip (1891) | `texts/twisted-lip-pg1661.txt` | https://www.gutenberg.org/cache/epub/1661/pg1661.txt | `6.<단락>` (『모험』 6번째 단편) |

정답 셋 코퍼스다. 이 작품군은 결함이 **있어야** 합격이다(정전의 유명한 모순 두 가지):

1. 왓슨의 부상 부위: 『주홍색 연구』 1.1.2 "struck on the shoulder by a Jezail bullet" ↔ 『네 사람의 서명』 1.17
   "sat nursing my wounded leg. I had a Jezail bullet through it".
2. 왓슨의 이름: 『주홍색 연구』 1.0.2 "Reminiscences of JOHN H. WATSON, M.D." ↔ 「입술이 비뚤어진 사나이」 6.9 아내가
   "should you rather that I sent James off to bed?".

이야기 순서(원문 근거, 모듈을 건너는 `when after`):
『주홍색 연구』 아프간 원정(1.1.1-2) → 귀국, 스탬퍼드, 홈즈를 만나 베이커 가에 함께 산다(1.1.5-1.2.1) →
『네 사람의 서명』 "the years that I had lived with him in Baker Street"(1.17), 메리 모스탄이 찾아온다(1.55, 2.1), 청혼(11.40-41),
약혼을 홈즈에게 알린다(12.117) → 「입술」 "One night—it was in June, ’89"(6.2), "my wife"(6.2-6.11) — 결혼 뒤다.
그래서 나중 단언은 다리(leg)와 James 이고, 진단 대상도 그 둘이다(규칙: 시간상 나중 단언이 대상).

포팅 범위: 두 단서를 담은 장면과 그것을 잇는 최소한의 정보 흐름 — 원정(무대 밖), 크라이테리온 바·홀번(스탬퍼드),
병원 실험실(첫 만남, "You have been in Afghanistan, I perceive"), 3월 4일 아침(추리의 설명, 해병 하사관), 서명 1-2장
(베이커 가, 메리의 의뢰), 11장(빈 상자, 청혼), 12장 끝(약혼 통고), 입술 6.2-6.33(케이트 휘트니, 아편굴의 홈즈).

```sl
work holmes "Sherlock Holmes — Watson's wound and Watson's name"
  unit day
  name ko "셜록 홈즈 — 왓슨의 부상과 이름"
  name en "Sherlock Holmes — Watson's wound and Watson's name"

character watson "Watson", "Dr. Watson", "John H. Watson" major
  believes watson_seeks_rooms                 # 1.1.4 "to take up my quarters in some less pretentious and less expensive domicile"
  name ko "왓슨"
  name en "Dr. Watson"
character holmes "Sherlock Holmes", "Holmes" major
  believes holmes_seeks_partner               # 1.1.12 "he could not get someone to go halves with him"
  believes baker_street_suite                 # 1.1.52 "I have my eye on a suite in Baker Street"
  name ko "셜록 홈즈"
  name en "Sherlock Holmes"
character stamford "Stamford", "young Stamford" minor
  name ko "스탬퍼드"
  name en "Stamford"
character murray "Murray" extra
  name ko "머리"
  name en "Murray"
character commissionaire "the commissionaire" extra
  believes messenger_was_marine               # 1.2.50 "A sergeant, sir, Royal Marine Light Infantry"
  name ko "심부름꾼"
  name en "the commissionaire"
character mary "Mary Morstan", "Miss Morstan" major
  name ko "메리 모스탄"
  name en "Mary Morstan"
character kate "Kate Whitney" minor
  name ko "케이트 휘트니"
  name en "Kate Whitney"
character isa "Isa Whitney" minor
  name ko "아이자 휘트니"
  name en "Isa Whitney"

place maiwand "the field of Maiwand"
  name ko "마이완드 전장"
  name en "Maiwand"
place holborn "the Criterion Bar and the Holborn"
  name ko "크라이테리온 바와 홀번 식당"
  name en "the Holborn"
place laboratory "the chemical laboratory at Barts"
  name ko "바츠 병원 화학 실험실"
  name en "the chemical laboratory"
place baker_street "the sitting-room at 221B Baker Street"
  name ko "베이커 가 221B 거실"
  name en "221B Baker Street"
place forrester_house "Mrs. Cecil Forrester's house"
  name ko "세실 포레스터 부인의 집"
  name en "Mrs. Forrester's"
place watson_home "the Watsons' sitting-room"
  name ko "왓슨 부부의 거실"
  name en "our sitting-room"
place bar_of_gold "the Bar of Gold, Upper Swandam Lane"
  name ko "어퍼 스완담 골목의 바 오브 골드"
  name en "the Bar of Gold"

fact watson_wounded "Watson was hit by a Jezail bullet at Maiwand" true
fact watson_from_afghanistan "Watson has come from Afghanistan" true
fact watson_seeks_rooms "Watson is looking for lodgings" true
fact holmes_seeks_partner "Holmes wants someone to go halves with him in rooms" true
fact baker_street_suite "Holmes has his eye on a suite in Baker Street" true
fact messenger_was_marine "The messenger was a retired sergeant of Marines" true
fact watson_sent_telegram "Watson sent a telegram from the Wigmore Street Post-Office this morning" true
fact pearls_sent "Every year since 1882 a pearl has come to Mary Morstan" true
fact treasure_lost "The Agra treasure box is empty" true
fact engaged "Watson and Mary Morstan are engaged" true
fact isa_missing "Isa Whitney has not been home for two days" true
fact isa_at_den "Isa Whitney is at the Bar of Gold in Upper Swandam Lane" true
fact holmes_in_den "The old man by the brazier is Sherlock Holmes in disguise" true
```
