package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	browser "github.com/EDDYCJY/fake-useragent"
	"github.com/tidwall/gjson"
)

var api_url = "https://lotm.otherside.xyz/api/trpc"
var api_url2 = "https://api.helika.io/v1"

var auth = ""
var processLog = 80.0

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	for {

		buf, err := ioutil.ReadFile("./config.json")
		if err != nil {
			log.Println(err)
			return
		}
		config := string(buf)
		auth = gjson.Get(config, "auth").String()
		monitorIntervalMintues := gjson.Get(config, "monitorIntervalMintues").Int()
		processLog = gjson.Get(config, "processLog").Float()

		startMonitor()
		time.Sleep(time.Minute * time.Duration(monitorIntervalMintues))
	}
}

func startMonitor() {
	datas, err := GetDefaultData("GET", fmt.Sprintf(`%s/currency.getMystBalance,otherdeed.getOtherdeeds,oda.getOdaInventory?batch=1&input={"0":{"json":null,"meta":{"values":["undefined"]}},"1":{"json":null,"meta":{"values":["undefined"]}},"2":{"json":null,"meta":{"values":["undefined"]}}}`, api_url), "", map[string]string{})
	if err != nil {
		log.Println(err)
		return
	}

	arraies := make([]map[string]interface{}, 0, 3)
	err = json.Unmarshal(datas, &arraies)
	if err != nil {
		log.Println(err)
		return
	}
	if len(arraies) != 3 {
		log.Println(string(datas))
		return
	}

	oneBytes, err := json.Marshal(arraies[1])
	if err != nil {
		log.Println(err)
		return
	}
	ides := gjson.GetBytes(oneBytes, "result.data.json").Array()

	if len(ides) == 0 {
		log.Println(string(oneBytes))
		return
	}

	log.Println("otherdeed数量:", len(ides))

	processes := map[int]float64{}
	odaTokenIdes := map[string]int{}
	for _, idInfo := range ides {
		isLegacy := idInfo.Get("isLegacy").Bool()
		if isLegacy {
			continue
		}
		otherdeedId := int(idInfo.Get("id").Int())
		envTier := idInfo.Get("envTier").Int()
		envSlots := idInfo.Get("envSlots").Array()
		hunters := 0
		for _, slot := range envSlots {
			role := slot.Get("role").String()
			if strings.EqualFold(role, "Hunter") {
				hunters++
			}
		}
		// if envTier == 2 && hunters == 0 {
		// 	log.Printf("https://lotm.otherside.xyz/camp/otherdeed/%d", otherdeedId)
		// }
		process := idInfo.Get("goliathProgress").Float()
		process = 100 - process*100
		processes[otherdeedId] = process

		time.Sleep(time.Millisecond * 100)
		sessionBytes, err := GetDefaultData("POST", api_url+"/land.login", `{
			"json": {
			  "fightLogDuration": 300000
			}
		  }`, map[string]string{
			"X-Land-Token-Id": strconv.Itoa(otherdeedId),
		})

		if err != nil {
			log.Println(err)
			continue
		}

		if bytes.Contains(sessionBytes, []byte("doesn't have any Hunters")) {
			continue
		}

		sessionToken := gjson.GetBytes(sessionBytes, "result.data.json.sessionToken").String()
		sessionId := gjson.GetBytes(sessionBytes, "result.data.json.sessionId").String()
		walletAddress := gjson.GetBytes(sessionBytes, "result.data.json.walletAddress").String()
		//tier := gjson.GetBytes(sessionBytes, "result.data.json.landData.tier").Int()
		status := gjson.GetBytes(sessionBytes, "result.data.json.status").String()
		//season_id := gjson.GetBytes(sessionBytes, "result.data.json.currentSeason.id").String()
		fight_id := gjson.GetBytes(sessionBytes, "result.data.json.fightSession.id").Int()
		goliath := gjson.GetBytes(sessionBytes, "result.data.json.goliath.code").String()
		startTimestamp := gjson.GetBytes(sessionBytes, "result.data.json.fightSession.startDate").Int() / 1000

		if len(sessionToken) == 0 {
			log.Println("not found sessionToken from:", string(sessionBytes))
			continue
		}
		abilityCount := 0
		abilityIdes := map[string][]string{}
		creatures := gjson.GetBytes(sessionBytes, "result.data.json.creatures").Array()
		cooldownAbilitess := map[string]map[string]bool{}
		for _, creature := range creatures {
			datas := creature.Get("data").Array()
			for _, data := range datas {
				creatureId := data.Get("id").String()
				odaTokenId := data.Get("odaTokenId").Int()
				abilities := data.Get("abilities").Array()
				abilityCount += len(abilities)

				odaTokenIdes[creatureId] = int(odaTokenId)
				abilityIdes[creatureId] = []string{}
				for _, ability := range abilities {
					abilityId := ability.Get("id").String()
					abilityIdes[creatureId] = append(abilityIdes[creatureId], abilityId)
				}
			}

			cooldowns := creature.Get("abilityCooldowns").Array()
			for _, cooldown := range cooldowns {
				cardId := cooldown.Get("cardId").String()
				abilityId := cooldown.Get("abilityId").String()

				if cooldownAbilitess[cardId] == nil {
					cooldownAbilitess[cardId] = map[string]bool{}
				}
				cooldownAbilitess[cardId][abilityId] = true
			}

		}
		if len(status) == 0 {
			log.Println(string(sessionBytes))
			continue
		}
		if !strings.EqualFold(status, "ON_GOING") {
			log.Printf("[%d]status is %s, 游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", otherdeedId, status, otherdeedId)
			continue
		}

		//2023-12-06T02:54:49.418Z
		date := time.Now().UTC().Format("2006-01-02T15:04:05.999Z")
		//log.Println("date", date)
		gameStartRes, err := GetDefaultData("POST", api_url2+"/game/game-event", fmt.Sprintf(`{
			"events": [
			  {
				"created_at": "%s",
				"game_id": "shattered",
				"event_type": "session",
				"event": {
				  "event_sub_type": "session_start",
				  "wallet_address": "%s",
				  "data": {
					"session_id": "%s",
					"fight_id": %d,
					"system_spec": {
					  "browser": {
						"name": "Chrome",
						"version": "120.0.0.0",
						"major": "120"
					  },
					  "device": {
						"model": "Macintosh",
						"vendor": "Apple"
					  },
					  "engine": {
						"name": "Blink",
						"version": "120.0.0.0"
					  },
					  "os": {
						"name": "macOS",
						"version": "10.15.7"
					  }
					}
				  }
				}
			  }
			]
		  }`, date, walletAddress, sessionId, fight_id), map[string]string{
			"Origin":         "https://lotm.otherside.xyz",
			"Referer":        "https://lotm.otherside.xyz",
			"Sec-Fetch-Mode": "cors",
			"Sec-Fetch-Site": "cross-site",
			"X-Api-Key":      "cee8a11e05226fb66ba2bc442fe036",
		})

		if err != nil {
			log.Println(err)
			continue
		}
		status2 := gjson.GetBytes(gameStartRes, "status").String()
		if !strings.EqualFold(status2, "ok") {
			log.Println(string(gameStartRes))
			continue
		}

		for cardId, abilities := range abilityIdes {
			for _, abilityId := range abilities {
				if cooldownAbilitess[cardId] == nil || !cooldownAbilitess[cardId][abilityId] {
					//if strings.Contains(abilityId, "direct_damage") || strings.Contains(abilityId, "damage_over_time") {
					time.Sleep(time.Millisecond * 100)
					gamePlayInfo, err := GetDefaultData("POST", api_url+"/gameplay.cast", fmt.Sprintf(`{
						"json": {
						  "sessionToken": "%s",
						  "creatureId": %s,
						  "abilityId": "%s",
						  "fightLogDuration": 300000
						}
					  }`, sessionToken, cardId, abilityId), map[string]string{
						"X-Land-Token-Id": strconv.Itoa(otherdeedId),
					})
					if err != nil {
						log.Println(err)
						continue
					}

					serverProcessTimeMillisecond := gjson.GetBytes(gamePlayInfo, "result.data.json.serverProcessTimeMillisecond").Int()
					if serverProcessTimeMillisecond == 0 {
						log.Printf("[%d]-[%d]运行技能[%s]失败，游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", otherdeedId, odaTokenIdes[cardId], abilityId, otherdeedId)
						log.Println(string(gamePlayInfo))
					} else {
						log.Printf("[%d]-[%d]运行技能[%s]成功", otherdeedId, odaTokenIdes[cardId], abilityId)
					}
					// } else {
					// 	log.Printf("[%d]-[%d]技能[%s]可以运行，游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", otherdeedId, odaTokenIdes[cardId], abilityId, otherdeedId)

					// }
				}
			}
		}

		time.Sleep(time.Millisecond * 100)
		boxes, err := GetDefaultData("POST", api_url+"/treasure.getUnclaimedChests", fmt.Sprintf(`{
			"json": {
			  "sessionToken": "%s"
			}
		  }`, sessionToken), map[string]string{
			"X-Land-Token-Id": strconv.Itoa(otherdeedId),
		})
		if err != nil {
			log.Println(err)
			continue
		}
		if len(gjson.GetBytes(boxes, "result.data.json").Array()) > 0 {
			log.Printf("[%d]有未领取宝箱，游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", otherdeedId, otherdeedId)

			// url := fmt.Sprintf("https://lotm.otherside.xyz/shattered/otherdeed/%d", otherdeedId)
			// osType := runtime.GOOS
			// if osType == "windows" {
			// 	cmd := exec.Command(`cmd`, `/c`, `start`, url)
			// 	cmd.SysProcAttr = &syscall.SysProcAttr{Foreground: false}
			// 	cmd.Start()
			// } else if osType == "darwin" {
			// 	exec.Command(`open`, url).Start()
			// } else {
			// 	log.Println("unknown os")
			// }

			continue
		}

		// time.Sleep(time.Millisecond * 100)
		// sessionInfo, err := GetDefaultData("POST", api_url+"/land.getSessionById", fmt.Sprintf(`{
		// 	"json": {
		// 	  "sessionToken": "%s"
		// 	}
		//   }`, sessionToken), map[string]string{
		// 	"X-Land-Token-Id": strconv.Itoa(otherdeedId),
		// })
		// if err != nil {
		// 	log.Println(err)
		// 	continue
		// }

		// leaderBoard, err := GetDefaultData("GET", fmt.Sprintf(`%s/leaderboard.getLeaderboard?input={"json":{"sessionToken":"%s","seasonId":"%s","tier":%d,"walletAddress":"%s"}}`, api_url, sessionToken, season_id, tier, walletAddress), "", map[string]string{
		// 	"X-Land-Token-Id": strconv.Itoa(otherdeedId),
		// })
		// if err != nil {
		// 	log.Println(err)
		// 	continue
		// }

		// ranks := gjson.GetBytes(leaderBoard, "result.data.json.results").Array()
		// if len(ranks) == 0 {
		// 	log.Println(string(leaderBoard))
		// 	continue
		// }
		// for _, rank := range ranks {
		// 	players := rank.Get("players").Array()
		// 	for _, player := range players {
		// 		landTokenId := player.Get("landTokenId").Int()
		// 		if landTokenId != int64(otherdeedId) {
		// 			continue
		// 		}
		// 		process := player.Get("currentProgress").Float()
		// 		currentProgressTime := player.Get("currentProgressTime").Int()
		// 		totalKills := player.Get("totalKills").Int()
		// 		hours := currentProgressTime / 1000 / 60 / 60
		// 		if process > processLog /*|| totalKills > 0*/ {
		// 			log.Printf("[%d]关卡: %d, 进度: %.2f%%, 时间: %d天%d小时, 环境: %d", otherdeedId, totalKills+1, process, hours/24, hours%24, envTier)

		// 			break
		// 		}
		// 	}
		// }

		if process > processLog /*|| totalKills > 0*/ {
			remainingHour := int(float64(time.Now().Unix()-startTimestamp) * 100 / process / 3600)
			log.Printf("[%d]怪物: %s, 进度: %.2f%%, 环境: %d, 预估剩余时间: %d天%d小时, 开始时间：%s, ", otherdeedId, goliath, process, envTier, remainingHour/24, remainingHour%24, time.Unix(startTimestamp, 0).Format("2006-01-02T15:04:05"))
		}
	}

	log.Println("----------------------end-----------------------")
}

func GetDefaultData(method, _url, req string, headers map[string]string) ([]byte, error) {
	//log.Println(method, _url, req, headers)
	var bodyStr = req
	newReq, err := http.NewRequest(method, _url, strings.NewReader(bodyStr))
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	newReq.Header.Set("content-type", "application/json")
	newReq.Header.Set("user-agent", browser.MacOSX())
	newReq.Header.Set("Authorization", auth)

	for key, val := range headers {
		newReq.Header.Set(key, val)
	}

	var resBody *http.Response
	resBody, err = http.DefaultClient.Do(newReq)

	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	defer resBody.Body.Close()
	bodybuf, err := ioutil.ReadAll(resBody.Body)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return bodybuf, nil
}
