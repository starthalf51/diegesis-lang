---
module: bible.gospels
import: [bible.cast]
mode: authored
---

# 베드로의 부인 — 네 복음서의 기록

같은 두 사건을 네 복음서가 따로 전한다. 사건 하나에 기록(account) 넷.

1. `peter_foretold` — 예수가 베드로의 부인을 미리 말한다(마 26:34, 막 14:30, 눅 22:34, 요 13:38). 기록마다 그
   예언이 promise 복선이다. 마태·마가에서는 베드로가 예언을 듣고 곧 부정한다(마 26:35, 막 14:31 "I will not deny
   you") — 들은 것을 믿지 않는다(`doubts`). 누가·요한에서는 베드로의 장담(눅 22:33, 요 13:37)이 예언보다 앞선다.
2. `peter_denies` — 그날 밤 대제사장의 뜰. 묻는 사람, 베드로의 거짓말, 닭 울음, 기억.
   - 마가는 닭이 두 번 운다(14:68, 14:72 "the second time"). 예언도 "before the rooster crows twice"(14:30).
   - 요한에서 처음 묻는 사람은 베드로를 들여보낸 문지기 여종이다(18:16-17). 공관복음의 여종은 뜰에서 그에게 온다.
   - 누가만 "The Lord turned and looked at Peter"(22:61)를 적는다.
   - 요한은 베드로가 기억했다고 적지 않는다(18:27 은 닭 울음에서 끝난다).
   요한 18:19-24(안나스의 심문)은 베드로가 나오지 않아 포팅하지 않았다. 누가 22:54-55(뜰로 따라 들어감)는 산문만.

묻는 사람이 고발을 어디서 알았는지: 본 사람은 `sees`(마 26:71 "saw him", 막 14:67 "seeing Peter", 눅 22:56·58
"saw him"). 마 26:69 의 여종은 "came to him" 뿐이지만 고발("You were also with Jesus")이 알아봄을 전제한다 —
같은 절에 `sees` 를 둔다. 곁에 선 사람들은 여종의 말을 들었다(마 26:71 "said to those who were there", 막 14:69).

```sl
event peter_foretold "Jesus foretells that Peter will deny him"
  when after start

account matthew_foretells of peter_foretold by matthew
  peter tells jesus: "Even if all will be made to stumble because of you, I will never be made to stumble." src matt.26.33
  jesus tells peter: peter_will_deny "Most certainly I tell you that tonight, before the rooster crows, you will deny me three times." src matt.26.34
  setup matthew_foretold promise: peter_will_deny
  peter doubts peter_will_deny
  peter tells jesus: peter_ready_to_die "Even if I must die with you, I will not deny you." src matt.26.35

account mark_foretells of peter_foretold by mark
  peter tells jesus: "Although all will be offended, yet I will not." src mark.14.29
  jesus tells peter: peter_will_deny "Most certainly I tell you that you today, even this night, before the rooster crows twice, you will deny me three times." src mark.14.30
  setup mark_foretold promise: peter_will_deny
  peter doubts peter_will_deny
  peter tells jesus: peter_ready_to_die "If I must die with you, I will not deny you." src mark.14.31

account luke_foretells of peter_foretold by luke
  jesus tells peter: "Simon, Simon, behold, Satan asked to have all of you, that he might sift you as wheat" src luke.22.31
  peter tells jesus: peter_ready_to_die "Lord, I am ready to go with you both to prison and to death!" src luke.22.33
  jesus tells peter: peter_will_deny "I tell you, Peter, the rooster will by no means crow today until you deny that you know me three times." src luke.22.34
  setup luke_foretold promise: peter_will_deny

account john_foretells of peter_foretold by john
  peter asks jesus: "Lord, where are you going?" src john.13.36
  peter tells jesus: peter_ready_to_die "Lord, why can't I follow you now? I will lay down my life for you." src john.13.37
  jesus tells peter: peter_will_deny "Will you lay down your life for me? Most certainly I tell you, the rooster won't crow until you have denied me three times." src john.13.38
  setup john_foretold promise: peter_will_deny

event peter_denies "Peter denies Jesus three times"
  when after peter_foretold by 0h..1d                      # 마 26:34 "tonight", 막 14:30 "this night", 눅 22:34 "today"

account matthew_denial of peter_denies by matthew
  matt_maid sees: peter_with_jesus "a maid came to him" src matt.26.69
  matt_maid tells peter: peter_with_jesus "You were also with Jesus, the Galilean!" src matt.26.69
  peter lies matt_maid: not peter_with_jesus "I don't know what you are talking about." src matt.26.70
  matt_other_maid sees: peter_with_jesus "When he had gone out onto the porch, someone else saw him" src matt.26.71
  matt_other_maid tells matt_bystanders: peter_with_jesus "This man also was with Jesus of Nazareth." src matt.26.71
  peter lies matt_bystanders: not peter_knows_jesus "Again he denied it with an oath, I don't know the man." src matt.26.72
  matt_bystanders tells peter: peter_disciple "Surely you are also one of them, for your speech makes you known." src matt.26.73
  peter lies matt_bystanders: not peter_knows_jesus "Then he began to curse and to swear, I don't know the man!" src matt.26.74
  peter sees: rooster_crows "Immediately the rooster crowed." src matt.26.74
  peter believes peter_will_deny
  peter thinks: peter_will_deny "Peter remembered the word which Jesus had said to him, Before the rooster crows, you will deny me three times." src matt.26.75
  payoff matthew_foretold

account mark_denial of peter_denies by mark
  mark_maid sees: peter_with_jesus "seeing Peter warming himself, she looked at him" src mark.14.67
  mark_maid tells peter: peter_with_jesus "You were also with the Nazarene, Jesus!" src mark.14.67
  peter lies mark_maid: not peter_with_jesus "I neither know nor understand what you are saying." src mark.14.68
  peter sees: rooster_crows "He went out on the porch, and the rooster crowed." src mark.14.68
  mark_maid tells mark_bystanders: peter_disciple "This is one of them." src mark.14.69
  peter lies mark_bystanders: not peter_disciple "But he again denied it." src mark.14.70
  mark_bystanders tells peter: peter_disciple "You truly are one of them, for you are a Galilean, and your speech shows it." src mark.14.70
  peter lies mark_bystanders: not peter_knows_jesus "I don't know this man of whom you speak!" src mark.14.71
  peter sees: rooster_crows_twice "The rooster crowed the second time." src mark.14.72
  peter believes peter_will_deny
  peter thinks: peter_will_deny "Peter remembered the words that Jesus said to him, Before the rooster crows twice, you will deny me three times." src mark.14.72
  payoff mark_foretold

account luke_denial of peter_denies by luke
  luke_maid sees: peter_with_jesus "A certain servant girl saw him as he sat in the light, and looking intently at him" src luke.22.56
  luke_maid tells peter: peter_with_jesus "This man also was with him." src luke.22.56
  peter lies luke_maid: not peter_knows_jesus "Woman, I don't know him." src luke.22.57
  luke_man sees: peter_disciple "After a little while someone else saw him" src luke.22.58
  luke_man tells peter: peter_disciple "You also are one of them!" src luke.22.58
  peter lies luke_man: not peter_disciple "Man, I am not!" src luke.22.58
  luke_other sees: peter_with_jesus "for he is a Galilean" src luke.22.59   # 해석: 본문은 "확언했다"뿐, 갈릴리 사람임을 보고 알았다는 지각은 "for he is a Galilean" 에서 읽어 낸 것
  luke_other tells peter: peter_with_jesus "Truly this man also was with him, for he is a Galilean!" src luke.22.59
  peter lies luke_other: not peter_with_jesus "Man, I don't know what you are talking about!" src luke.22.60
  peter sees: rooster_crows "Immediately, while he was still speaking, a rooster crowed." src luke.22.60
  peter sees: jesus_looks_at_peter "The Lord turned and looked at Peter." src luke.22.61
  peter believes peter_will_deny
  peter thinks: peter_will_deny "Then Peter remembered the Lord's word, how he said to him, Before the rooster crows you will deny me three times." src luke.22.61
  payoff luke_foretold

account john_denial of peter_denies by john
  other_disciple does bring_in peter with doorkeeper_admits_peter "So the other disciple, who was known to the high priest, went out and spoke to her who kept the door, and brought in Peter." src john.18.16
  john_doorkeeper asks peter: peter_disciple "Are you also one of this man's disciples?" src john.18.17
  peter lies john_doorkeeper: not peter_disciple "I am not." src john.18.17
  john_servants asks peter: peter_disciple "You aren't also one of his disciples, are you?" src john.18.25
  peter lies john_servants: not peter_disciple "He denied it and said, I am not." src john.18.25
  malchus_relative asks peter: peter_with_jesus "Didn't I see you in the garden with him?" src john.18.26
  peter lies malchus_relative: not peter_with_jesus "Peter therefore denied it again" src john.18.27
  peter sees: rooster_crows "and immediately the rooster crowed." src john.18.27
  payoff john_foretold
```

마태 26:69 — 베드로는 바깥뜰에 앉아 있었다. 마가 14:66 — 아랫뜰. 누가 22:54-55 — 베드로는 멀찍이 따라가 뜰 가운데
피운 불 곁에 앉는다. 요한 18:15-18 — 다른 제자가 대제사장과 아는 사이라 문지기에게 말해 베드로를 들인다.
