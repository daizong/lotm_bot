package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	browser "github.com/EDDYCJY/fake-useragent"
	"github.com/tidwall/gjson"
)

var api_url = "https://lotm.otherside.xyz/api/trpc"

var auth = ""
var wallet_addr = ""
var processLog = 80.0

func main() {
	log.SetFlags(log.Ldate | log.Ltime /*| log.Lshortfile*/)

	buf, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Println(err)
		return
	}
	config := string(buf)
	auth = gjson.Get(config, "auth").String()
	wallet_addr = gjson.Get(config, "wallet").String()
	monitorIntervalMintues := gjson.Get(config, "monitorIntervalMintues").Int()
	processLog = gjson.Get(config, "processLog").Float()

	for {
		startMonitor()
		time.Sleep(time.Minute * time.Duration(monitorIntervalMintues))
	}
}

func startMonitor() {
	processes := map[int]float64{}

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
	for _, idInfo := range ides {
		id := int(idInfo.Get("id").Int())
		envTier := idInfo.Get("envTier").Int()
		envSlots := idInfo.Get("envSlots").Array()
		hunters := 0
		for _, slot := range envSlots {
			role := slot.Get("role").String()
			if strings.EqualFold(role, "Hunter") {
				hunters++
			}
		}
		process := idInfo.Get("goliathProgress").Float()
		processes[id] = process

		sessionBytes, err := GetDefaultData("POST", api_url+"/land.login", `{
			"json": null,
			"meta": {
			  "values": [
				"undefined"
			  ]
			}
		  }`, map[string]string{
			"X-Land-Token-Id": strconv.Itoa(id),
		})

		if err != nil {
			log.Println(err)
			continue
		}

		if bytes.Contains(sessionBytes, []byte("doesn't have any Hunters")) {
			continue
		}

		sessionToken := gjson.GetBytes(sessionBytes, "result.data.json.sessionToken").String()
		tier := gjson.GetBytes(sessionBytes, "result.data.json.landData.tier").Int()
		if len(sessionToken) == 0 {
			log.Println("not found sessionToken from:", string(sessionBytes))
			continue
		}
		abilityCount := 0
		abilityIdes := map[string][]string{}
		creatures := gjson.GetBytes(sessionBytes, "result.data.json.creatures").Array()
		for _, creature := range creatures {
			creatureId := creature.Get("id").String()
			abilities := creature.Get("abilities").Array()
			abilityCount += len(abilities)

			abilityIdes[creatureId] = []string{}
			for _, ability := range abilities {
				abilityId := ability.Get("id").String()
				abilityIdes[creatureId] = append(abilityIdes[creatureId], abilityId)
			}
		}

		boxes, err := GetDefaultData("POST", api_url+"/treasure.getUnclaimedChests", fmt.Sprintf(`{
			"json": {
			  "sessionToken": "%s"
			}
		  }`, sessionToken), map[string]string{
			"X-Land-Token-Id": strconv.Itoa(id),
		})
		if err != nil {
			log.Println(err)
			continue
		}
		if len(gjson.GetBytes(boxes, "result.data.json").Array()) > 0 {
			log.Printf("[%d]有未领取宝箱，游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", id, id)

			url := fmt.Sprintf("https://lotm.otherside.xyz/shattered/otherdeed/%d", id)
			osType := runtime.GOOS
			if osType == "windows" {
				cmd := exec.Command(`cmd`, `/c`, `start`, url)
				cmd.SysProcAttr = &syscall.SysProcAttr{Foreground: false}
				cmd.Start()
			} else if osType == "darwin" {
				exec.Command(`open`, url).Start()
			} else {
				log.Println("unknown os")
			}

			continue
		}

		sessionInfo, err := GetDefaultData("POST", api_url+"/land.getSessionById", fmt.Sprintf(`{
			"json": {
			  "sessionToken": "%s"
			}
		  }`, sessionToken), map[string]string{
			"X-Land-Token-Id": strconv.Itoa(id),
		})
		if err != nil {
			log.Println(err)
			continue
		}
		status := gjson.GetBytes(sessionInfo, "result.data.json.status").String()
		season_id := gjson.GetBytes(sessionInfo, "result.data.json.season_id").String()
		creature_ability_cooldowns := gjson.GetBytes(sessionInfo, "result.data.json.creature_ability_cooldowns").Array()
		if !strings.EqualFold(status, "ON_GOING") {
			log.Println(string(sessionInfo))
			log.Printf("[%d]status is %s, 游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", id, status, id)
			continue
		}

		cooldownAbilitess := map[string]map[string]bool{}
		if len(creature_ability_cooldowns) > 0 {
			for _, cooldown := range creature_ability_cooldowns {
				cardId := cooldown.Get("cardId").String()
				abilityId := cooldown.Get("abilityId").String()

				if cooldownAbilitess[cardId] == nil {
					cooldownAbilitess[cardId] = map[string]bool{}
				}
				cooldownAbilitess[cardId][abilityId] = true
			}
		}
		for cardId, abilities := range abilityIdes {
			for _, abilityId := range abilities {
				if cooldownAbilitess[cardId] == nil || !cooldownAbilitess[cardId][abilityId] {
					time.Sleep(time.Millisecond * 100)
					gamePlayInfo, err := GetDefaultData("POST", api_url+"/gameplay.cast", fmt.Sprintf(`{
						"json": {
						  "sessionToken": "%s",
						  "creatureId": %s,
						  "abilityId": "%s",
						  "fightLogDuration": 300000
						}
					  }`, sessionToken, cardId, abilityId), map[string]string{
						"X-Land-Token-Id": strconv.Itoa(id),
					})
					if err != nil {
						log.Println(err)
						continue
					}

					serverProcessTimeMillisecond := gjson.GetBytes(gamePlayInfo, "result.data.json.serverProcessTimeMillisecond").Int()
					if serverProcessTimeMillisecond == 0 {
						log.Printf("运行[%s]技能[%s]失败，游戏链接: https://lotm.otherside.xyz/shattered/otherdeed/%d", cardId, abilityId, id)
						log.Println(string(gamePlayInfo))
					} else {
						log.Printf("运行[%s]技能[%s]成功", cardId, abilityId)
					}
				}
			}
		}

		leaderBoard, err := GetDefaultData("GET", fmt.Sprintf(`%s/leaderboard.getLeaderboard?input={"json":{"sessionToken":"%s","seasonId":"%s","tier":%d}}`, api_url, sessionToken, season_id, tier), "", map[string]string{
			"X-Land-Token-Id": strconv.Itoa(id),
		})
		if err != nil {
			log.Println(err)
			continue
		}

		ranks := gjson.GetBytes(leaderBoard, "result.data.json").Array()
		if len(ranks) == 0 {
			log.Println(leaderBoard)
			continue
		}
		for _, rank := range ranks {
			players := rank.Get("players").Array()
			for _, player := range players {
				walletAddress := player.Get("walletAddress").String()
				process := player.Get("currentProgress").Float()
				currentProgressTime := player.Get("currentProgressTime").Int()
				totalKills := player.Get("totalKills").Int()
				hours := currentProgressTime / 1000 / 60 / 60
				if strings.EqualFold(walletAddress, wallet_addr) && (process > processLog /*|| totalKills > 0*/) {
					log.Printf("[%d]关卡: %d, 进度: %.2f%%, 时间: %d天%d小时, 环境: %d", id, totalKills+1, process, hours/24, hours%24, envTier)

					break
				}
			}
		}

	}

	log.Println("----------------------end-----------------------")
}

func GetDefaultData(method, _url, req string, headers map[string]string) ([]byte, error) {
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