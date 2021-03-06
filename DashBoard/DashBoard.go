package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	_ "github.com/lib/pq"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/pubsub"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/mediocregopher/radix.v2/sentinel"
	"github.com/mediocregopher/radix.v2/util"
)

var sentinelPool *sentinel.Client

var redisPool *pool.Pool
var statClient *StatsdClient

const layout = "2006-01-02T15:04:05Z07:00"

var dashboardMetaInfo []MetaData

//var log = log4go.NewLogger()

func errHndlr(errorFrom, command string, err error) {
	if err != nil {
		fmt.Println("error:", errorFrom, ":: ", command, ":: ", err)
	}
}

func InitiateStatDClient() {
	host := statsDIp
	port := statsDPort

	//client := statsd.New(host, port)
	statClient = New(host, port)
}

func InitiateRedis() {

	var err error

	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		if err = client.Cmd("AUTH", redisPassword).Err; err != nil {
			client.Close()
			return nil, err
		}
		if err = client.Cmd("select", redisDb).Err; err != nil {
			client.Close()
			return nil, err
		}
		return client, nil
	}

	if redisMode == "sentinel" {
		sentinelIps := strings.Split(sentinelHosts, ",")

		if len(sentinelIps) > 1 {
			sentinelIp := fmt.Sprintf("%s:%s", sentinelIps[0], sentinelPort)
			sentinelPool, err = sentinel.NewClientCustom("tcp", sentinelIp, 10, df, redisClusterName)

			if err != nil {
				errHndlr("InitiateRedis", "InitiateSentinel", err)
			}
		} else {
			fmt.Println("Not enough sentinel servers")
		}
	} else {
		redisPool, err = pool.NewCustom("tcp", redisIp, 10, df)

		if err != nil {
			errHndlr("InitiateRedis", "InitiatePool", err)
		}
	}

}

func PubSub() {
	var c2 *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in PubSub", r)
		}

		if c2 != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, c2)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()
	//c2, err := redisPubPool.Get()
	//errHndlr("getConnFromPool", err)
	//defer redisPubPool.Put(c2)

	//client, err := sentinel.NewClient("tcp", redisPubSubIp, 10, redisClusterName)
	//errHndlr("Sentinal", err)

	//c2, err2 := client.GetMaster(redisClusterName)
	//errHndlr("Dial tcp", err2)
	//defer client.PutMaster(redisClusterName, c2)

	for {

		if redisMode == "sentinel" {

			c2, err = sentinelPool.GetMaster(redisClusterName)
			errHndlr("PubSub", "getConnFromPool", err)
			//defer sentinelPool.PutMaster(redisClusterName, c2)

			if err == nil {
				psc := pubsub.NewSubClient(c2)
				psr := psc.Subscribe("events")
				ppsr := psc.PSubscribe("EVENT:*")

				fmt.Println("Event Start")

				if ppsr.Err == nil {

					for {
						psr = psc.Receive()

						if psr.Timeout() {
							fmt.Println("psc.Receive Timeout:: ", psr.Timeout())
							break

						}
						if psr.Err != nil {

							fmt.Println("psc.Receive Err:: ", psr.Err.Error())
							break
						}
						list := strings.Split(psr.Message, ":")
						fmt.Println(list)
						if len(list) >= 9 {
							stenent := list[1]
							scompany := list[2]
							sbusinessUnit := list[3]
							sclass := list[4]
							stype := list[5]
							scategory := list[6]
							sparam1 := list[7]
							sparam2 := list[8]
							ssession := list[9]

							itenet, _ := strconv.Atoi(stenent)
							icompany, _ := strconv.Atoi(scompany)

							go OnEvent(itenet, icompany, sbusinessUnit, sclass, stype, scategory, ssession, sparam1, sparam2, "")
						}

					}
					//s := strings.Split("127.0.0.1:5432", ":")
				} else {
					fmt.Println("ppsr Err:: ", ppsr.Err.Error())
				}

				fmt.Println("Unsubscribe")
				psc.Unsubscribe("events")
			}

		} else {

			c2, err = redis.Dial("tcp", redisPubSubIp)
			errHndlr("PubSub", "dial", err)
			defer c2.Close()

			//authServer

			if err == nil {
				authE := c2.Cmd("auth", redisPassword)
				errHndlr("PubSub", "auth", authE.Err)

				if authE.Err == nil {

					psc := pubsub.NewSubClient(c2)
					psr := psc.Subscribe("events")
					ppsr := psc.PSubscribe("EVENT:*")

					fmt.Println("Event Start")

					if ppsr.Err == nil {

						for {
							psr = psc.Receive()

							if psr.Timeout() {
								fmt.Println("psc.Receive Timeout:: ", psr.Timeout())
								break

							}

							if psr.Err != nil {

								fmt.Println("psc.Receive Err:: ", psr.Err.Error())
								break
							}
							list := strings.Split(psr.Message, ":")
							fmt.Println(list)
							if len(list) >= 9 {
								stenent := list[1]
								scompany := list[2]
								sbusinessUnit := list[3]
								sclass := list[4]
								stype := list[5]
								scategory := list[6]
								sparam1 := list[7]
								sparam2 := list[8]
								ssession := list[9]

								itenet, _ := strconv.Atoi(stenent)
								icompany, _ := strconv.Atoi(scompany)

								go OnEvent(itenet, icompany, sbusinessUnit, sclass, stype, scategory, ssession, sparam1, sparam2, "")
							}

						}
						//s := strings.Split("127.0.0.1:5432", ":")
					}

					fmt.Println("Unsubscribe")
					psc.Unsubscribe("events")
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

}

func PersistsSummaryData(_summary SummeryDetail) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in PersistsSummaryData", r)
		}
	}()
	conStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=disable", pgUser, pgPassword, pgDbname, pgHost, pgPort)
	db, err := sql.Open("postgres", conStr)
	if err != nil {
		fmt.Println(err.Error())
	}

	result, err1 := db.Exec("INSERT INTO \"Dashboard_DailySummaries\"(\"Company\", \"Tenant\", \"WindowName\", \"BusinessUnit\", \"Param1\", \"Param2\", \"MaxTime\", \"TotalCount\", \"TotalTime\", \"ThresholdValue\", \"SummaryDate\", \"createdAt\", \"updatedAt\") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)", _summary.Company, _summary.Tenant, _summary.BusinessUnit, _summary.WindowName, _summary.Param1, _summary.Param2, _summary.MaxTime, _summary.TotalCount, _summary.TotalTime, _summary.ThresholdValue, _summary.SummaryDate, time.Now().Local(), time.Now().Local())
	if err1 != nil {
		fmt.Println(err1.Error())
	} else {
		fmt.Println("PersistsSummaryData: ", result)
		lInsertedId, err2 := result.LastInsertId()
		fmt.Println(err2)
		fmt.Println("Last inserted Id: ", lInsertedId)
	}
	db.Close()
}

func PersistsThresholdBreakDown(_summary ThresholdBreakDownDetail) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in PersistsThresholdBreakDown", r)
		}
	}()
	conStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=disable", pgUser, pgPassword, pgDbname, pgHost, pgPort)
	db, err := sql.Open("postgres", conStr)
	if err != nil {
		fmt.Println(err.Error())
	}

	result, err1 := db.Exec("INSERT INTO \"Dashboard_ThresholdBreakDowns\"(\"Company\", \"Tenant\", \"WindowName\", \"BusinessUnit\", \"Param1\", \"Param2\", \"BreakDown\", \"ThresholdCount\", \"SummaryDate\", \"Hour\", \"createdAt\", \"updatedAt\") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", _summary.Company, _summary.Tenant, _summary.BusinessUnit, _summary.WindowName, _summary.Param1, _summary.Param2, _summary.BreakDown, _summary.ThresholdCount, _summary.SummaryDate, _summary.Hour, time.Now().Local(), time.Now().Local())
	if err1 != nil {
		fmt.Println(err1.Error())
	} else {
		fmt.Println("PersistsThresholdBreakDown: ", result)
		lInsertedId, err2 := result.LastInsertId()
		fmt.Println(err2)
		fmt.Println("Last inserted Id: ", lInsertedId)
	}
	db.Close()
}

func PersistsMetaData(_class, _type, _category, _window string, count int, _flushEnable, _useSession, _persistSession, _thresholdEnable bool, _thresholdValue int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in PersistsMetaData", r)
		}
	}()
	conStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=disable", pgUser, pgPassword, pgDbname, pgHost, pgPort)
	db, err := sql.Open("postgres", conStr)
	if err != nil {
		fmt.Println(err.Error())
	}

	result, err1 := db.Exec("INSERT INTO \"Dashboard_MetaData\"(\"EventClass\", \"EventType\", \"EventCategory\", \"WindowName\", \"Count\", \"FlushEnable\", \"UseSession\", \"PersistSession\", \"ThresholdEnable\", \"ThresholdValue\", \"createdAt\", \"updatedAt\") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)", _class, _type, _category, _window, count, _flushEnable, _useSession, _persistSession, _thresholdEnable, _thresholdValue, time.Now().Local(), time.Now().Local())
	if err1 != nil {
		fmt.Println(err1.Error())
	} else {
		fmt.Println("PersistsMetaData: ", result)
		lInsertedId, err2 := result.LastInsertId()
		fmt.Println(err2)
		fmt.Println("Last inserted Id: ", lInsertedId)
	}
	db.Close()
}

func ReloadMetaData(_class, _type, _category string) bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in ReloadMetaData", r)
		}
	}()
	var result bool
	conStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=disable", pgUser, pgPassword, pgDbname, pgHost, pgPort)
	db, err := sql.Open("postgres", conStr)
	if err != nil {
		fmt.Println(err.Error())
		result = false
	}

	var EventClass string
	var EventType string
	var EventCategory string
	var WindowName string
	var Count int
	var FlushEnable bool
	var PersistSession bool
	var UseSession bool
	var ThresholdEnable bool
	var ThresholdValue int

	err1 := db.QueryRow("SELECT \"EventClass\", \"EventType\", \"EventCategory\", \"WindowName\", \"Count\", \"FlushEnable\", \"UseSession\", \"PersistSession\", \"ThresholdEnable\", \"ThresholdValue\" FROM \"Dashboard_MetaData\" WHERE \"EventClass\"=$1 AND \"EventType\"=$2 AND \"EventCategory\"=$3", _class, _type, _category).Scan(&EventClass, &EventType, &EventCategory, &WindowName, &Count, &FlushEnable, &UseSession, &PersistSession, &ThresholdEnable, &ThresholdValue)
	switch {
	case err1 == sql.ErrNoRows:
		fmt.Println("No metaData with that ID.")
		result = false
	case err1 != nil:
		fmt.Println(err1.Error())
		result = false
	default:
		fmt.Printf("EventClass is %s\n", EventClass)
		fmt.Printf("EventType is %s\n", EventType)
		fmt.Printf("EventCategory is %s\n", EventCategory)
		fmt.Printf("WindowName is %s\n", WindowName)
		fmt.Printf("Count is %d\n", Count)
		fmt.Printf("FlushEnable is %t\n", FlushEnable)
		fmt.Printf("UseSession is %t\n", UseSession)
		fmt.Printf("PersistSession is %t\n", PersistSession)
		fmt.Printf("ThresholdEnable is %t\n", ThresholdEnable)
		fmt.Printf("ThresholdValue is %d\n", ThresholdValue)
		CacheMetaData(EventClass, EventType, EventCategory, WindowName, Count, FlushEnable, UseSession, PersistSession, ThresholdEnable, ThresholdValue)
		result = true
	}
	db.Close()
	return result
}

func ReloadAllMetaData() bool {
	fmt.Println("-------------------------ReloadAllMetaData----------------------")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in ReloadAllMetaData", r)
		}
	}()
	var result bool
	conStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=disable", pgUser, pgPassword, pgDbname, pgHost, pgPort)
	db, err := sql.Open("postgres", conStr)
	if err != nil {
		fmt.Println(err.Error())
		result = false
	}

	var EventClass string
	var EventType string
	var EventCategory string
	var WindowName string
	var Count int
	var FlushEnable bool
	var PersistSession bool
	var UseSession bool
	var ThresholdEnable bool
	var ThresholdValue int

	//err1 := db.QueryRow("SELECT \"EventClass\", \"EventType\", \"EventCategory\", \"WindowName\", \"Count\", \"FlushEnable\", \"UseSession\", \"ThresholdEnable\", \"ThresholdValue\" FROM \"Dashboard_MetaData\"").Scan(&EventClass, &EventType, &EventCategory, &WindowName, &Count, &FlushEnable, &UseSession, &ThresholdEnable, &ThresholdValue)
	dataRows, err1 := db.Query("SELECT \"EventClass\", \"EventType\", \"EventCategory\", \"WindowName\", \"Count\", \"FlushEnable\", \"UseSession\", \"PersistSession\", \"ThresholdEnable\", \"ThresholdValue\" FROM \"Dashboard_MetaData\"")
	switch {
	case err1 == sql.ErrNoRows:
		fmt.Println("No metaData with that ID.")
		result = false
	case err1 != nil:
		fmt.Println(err1.Error())
		result = false
	default:
		dashboardMetaInfo = make([]MetaData, 0)
		for dataRows.Next() {
			dataRows.Scan(&EventClass, &EventType, &EventCategory, &WindowName, &Count, &FlushEnable, &UseSession, &PersistSession, &ThresholdEnable, &ThresholdValue)

			fmt.Printf("EventClass is %s\n", EventClass)
			fmt.Printf("EventType is %s\n", EventType)
			fmt.Printf("EventCategory is %s\n", EventCategory)
			fmt.Printf("WindowName is %s\n", WindowName)
			fmt.Printf("Count is %d\n", Count)
			fmt.Printf("FlushEnable is %t\n", FlushEnable)
			fmt.Printf("UseSession is %t\n", UseSession)
			fmt.Printf("PersistSession is %t\n", PersistSession)
			fmt.Printf("ThresholdEnable is %t\n", ThresholdEnable)
			fmt.Printf("ThresholdValue is %d\n", ThresholdValue)

			if cacheMachenism == "redis" {
				CacheMetaData(EventClass, EventType, EventCategory, WindowName, Count, FlushEnable, UseSession, PersistSession, ThresholdEnable, ThresholdValue)
			} else {
				var mData MetaData
				mData.EventClass = EventClass
				mData.EventType = EventType
				mData.EventCategory = EventCategory
				mData.Count = Count
				mData.FlushEnable = FlushEnable
				mData.ThresholdEnable = ThresholdEnable
				mData.ThresholdValue = ThresholdValue
				mData.UseSession = UseSession
				mData.PersistSession = PersistSession
				mData.WindowName = WindowName

				dashboardMetaInfo = append(dashboardMetaInfo, mData)
			}
		}
		dataRows.Close()
		result = true
	}
	db.Close()
	fmt.Println("DashBoard MetaData:: ", dashboardMetaInfo)
	return result
}

func CacheMetaData(_class, _type, _category, _window string, count int, _flushEnable, _useSession, _persistSession, _thresholdEnable bool, _thresholdValue int) {
	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in CacheMetaData", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()
	_windowName := fmt.Sprintf("META:%s:%s:%s:WINDOW", _class, _type, _category)
	_incName := fmt.Sprintf("META:%s:%s:%s:COUNT", _class, _type, _category)
	_flushName := fmt.Sprintf("META:%s:%s:%s:FLUSH", _class, _type, _category)
	_useSessionName := fmt.Sprintf("META:%s:%s:%s:USESESSION", _class, _type, _category)
	_persistSessionName := fmt.Sprintf("META:%s:%s:%s:PERSISTSESSION", _class, _type, _category)
	_thresholdEnableName := fmt.Sprintf("META:%s:%s:%s:thresholdEnable", _class, _type, _category)

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("CacheMetaData", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("CacheMetaData", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	//client, err := redis.DialTimeout("tcp", redisIp, time.Duration(10)*time.Second)
	//errHndlr(err)
	//defer client.Close()

	/*//authServer
	authE := client.Cmd("auth", redisPassword)
	errHndlr("auth", authE.Err)
	// select database
	r := client.Cmd("select", redisDb)
	errHndlr("selectDb", r.Err)*/

	if _flushEnable == true {
		errHndlr("CacheMetaData", "Cmd", client.Cmd("setnx", _flushName, _window).Err)
	} else {
		errHndlr("CacheMetaData", "Cmd", client.Cmd("del", _flushName).Err)
	}

	if _thresholdEnable == true {
		errHndlr("CacheMetaData", "Cmd", client.Cmd("setnx", _thresholdEnableName, _thresholdValue).Err)
	} else {
		errHndlr("CacheMetaData", "Cmd", client.Cmd("del", _thresholdEnableName).Err)
	}

	errHndlr("CacheMetaData", "Cmd", client.Cmd("setnx", _useSessionName, strconv.FormatBool(_useSession)).Err)
	errHndlr("CacheMetaData", "Cmd", client.Cmd("setnx", _persistSessionName, strconv.FormatBool(_persistSession)).Err)
	errHndlr("CacheMetaData", "Cmd", client.Cmd("setnx", _windowName, _window).Err)
	errHndlr("CacheMetaData", "Cmd", client.Cmd("setnx", _incName, strconv.Itoa(count)).Err)
}

func OnMeta(_class, _type, _category, _window string, count int, _flushEnable, _useSession, _persistSession, _thresholdEnable bool, _thresholdValue int) {
	CacheMetaData(_class, _type, _category, _window, count, _flushEnable, _useSession, _persistSession, _thresholdEnable, _thresholdValue)
	PersistsMetaData(_class, _type, _category, _window, count, _flushEnable, _useSession, _persistSession, _thresholdEnable, _thresholdValue)
}

func OnEvent(_tenent, _company int, _businessUnit, _class, _type, _category, _session, _parameter1, _parameter2, eventTime string) {

	if _businessUnit == "" || _businessUnit == "*" {
		fmt.Println("Use Default BusinessUnit")
		_businessUnit = "default"
	}
	if _parameter2 == "" || _parameter2 == "*" {
		fmt.Println("Use Default Param2")
		_parameter2 = "param2"
	}
	if _parameter1 == "" || _parameter1 == "*" {
		fmt.Println("Use Default Param1")
		_parameter1 = "param1"
	}
	temp := fmt.Sprintf("Tenant:%d Company:%d BusinessUnit:%s Class:%s Type:%s Category:%s Session:%s Param1:%s Param2:%s", _tenent, _company, _businessUnit, _class, _type, _category, _session, _parameter1, _parameter2)
	fmt.Println("OnEvent: ", temp)

	location, _ := time.LoadLocation("Asia/Colombo")
	fmt.Println("location:: " + location.String())

	var tm time.Time
	if eventTime != "" {
		eTime, _ := time.Parse(layout, eventTime)
		tm = eTime.In(location)
	} else {
		tm = time.Now().In(location)
	}
	fmt.Println("eventTime:: " + tm.String())

	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnEvent", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnEvent", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnEvent", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	var window, sinc, useSession, persistSession, threshold string
	var iinc int
	var thresholdEnabled bool
	var _werr, _ierr, _userr, _peerr, _thresherr, berr error

	if cacheMachenism == "redis" {
		fmt.Println("---------------------Use Redis----------------------")

		_window := fmt.Sprintf("META:%s:%s:%s:WINDOW", _class, _type, _category)
		_inc := fmt.Sprintf("META:%s:%s:%s:COUNT", _class, _type, _category)
		_useSessionName := fmt.Sprintf("META:%s:%s:%s:USESESSION", _class, _type, _category)
		_persistSessionName := fmt.Sprintf("META:%s:%s:%s:PERSISTSESSION", _class, _type, _category)
		_thresholdEnableName := fmt.Sprintf("META:%s:%s:%s:thresholdEnable", _class, _type, _category)

		isWindowExist, windowExistErr := client.Cmd("exists", _window).Int()
		errHndlr("OnEvent", "Cmd windowExistErr", windowExistErr)
		isIncExist, incExistErr := client.Cmd("exists", _inc).Int()
		errHndlr("OnEvent", "Cmd incExistErr", incExistErr)

		if isWindowExist == 0 || isIncExist == 0 {
			ReloadMetaData(_class, _type, _category)
		}
		window, _werr = client.Cmd("get", _window).Str()
		errHndlr("OnEvent", "cmdGet", _werr)
		sinc, _ierr = client.Cmd("get", _inc).Str()
		errHndlr("OnEvent", "cmdGet", _ierr)
		useSession, _userr = client.Cmd("get", _useSessionName).Str()
		errHndlr("OnEvent", "cmdGet", _userr)
		persistSession, _peerr = client.Cmd("get", _persistSessionName).Str()
		errHndlr("OnEvent", "cmdGet", _peerr)
		threshold, _thresherr = client.Cmd("get", _thresholdEnableName).Str()
		errHndlr("OnEvent", "cmdGet", _thresherr)

		if threshold != "" {
			thresholdEnabled = true
		} else {
			thresholdEnabled = false
		}

		iinc, berr = strconv.Atoi(sinc)

	} else {
		fmt.Println("---------------------Use Memoey----------------------")
		for _, dmi := range dashboardMetaInfo {
			if dmi.EventClass == _class && dmi.EventType == _type && dmi.EventCategory == _category {
				window = dmi.WindowName
				iinc = dmi.Count
				useSession = strconv.FormatBool(dmi.UseSession)
				persistSession = strconv.FormatBool(dmi.PersistSession)
				threshold = strconv.Itoa(dmi.ThresholdValue)
				thresholdEnabled = dmi.ThresholdEnable
				break
			}
		}
	}

	fmt.Println("Session: %s iinc value is %d", _session, iinc)

	if _werr == nil && _ierr == nil && berr == nil {

		var statsDPath string
		switch _class {
		case "TICKET":
			statsDPath = "ticket"
			break

		default:
			statsDPath = "common"
			break
		}

		if iinc > 0 {

			if useSession == "true" {
				if persistSession == "true" {
					PersistSessionInfo(_tenent, _company, _businessUnit, window, _session, _parameter1, _parameter2, tm.Format(layout))
				} else {

					sessEventName := fmt.Sprintf("SESSION:%d:%d:%s:%s:%s:%s:%s", _tenent, _company, _businessUnit, window, _session, _parameter1, _parameter2)
					sessParamEventName := fmt.Sprintf("SESSIONPARAMS:%d:%d:%s:%s", _tenent, _company, window, _session)

					errHndlr("OnEvent", "Cmd sessEventName", client.Cmd("hset", sessEventName, "time", tm.Format(layout)).Err)
					errHndlr("OnEvent", "Cmd sessParamEventName", client.Cmd("hmset", sessParamEventName, "businessUnit", _businessUnit, "param1", _parameter1, "param2", _parameter2).Err)
				}
			}

			IncrementEvent(_tenent, _company, _businessUnit, window, _parameter1, _parameter2, statsDPath, tm)

		} else {

			if useSession == "true" {

				DecrementEvent(_tenent, _company, 0, window, _session, persistSession, statsDPath, threshold, tm, location, thresholdEnabled)

			} else {

				fmt.Println("Metadata not found for decriment: %s", _session)

			}

		}

	}

}

func IncrementEvent(_tenent, _company int, _businessUnit, window, _parameter1, _parameter2, statsDPath string, tm time.Time) {

	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in IncrementEvent", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnEvent", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnEvent", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	//------------------------Redis keys---------------------

	concEventName := fmt.Sprintf("CONCURRENT:%d:%d:%s:%s:%s", _tenent, _company, window, _parameter1, _parameter2)
	totCountEventName := fmt.Sprintf("TOTALCOUNT:%d:%d:%s:%s:%s", _tenent, _company, window, _parameter1, _parameter2)
	concEventName_businessUnit := fmt.Sprintf("CONCURRENT:%d:%d:%s:%s:%s:%s", _tenent, _company, _businessUnit, window, _parameter1, _parameter2)
	totCountEventName_businessUnit := fmt.Sprintf("TOTALCOUNT:%d:%d:%s:%s:%s:%s", _tenent, _company, _businessUnit, window, _parameter1, _parameter2)

	//totCountHrEventName := fmt.Sprintf("TOTALCOUNTHR:%d:%d:%s:%s:%s:%d:%d", _tenent, _company, window, _parameter1, _parameter2, tm.Hour(), tm.Minute())
	//totCountHrEventName_businessUnit := fmt.Sprintf("TOTALCOUNTHR:%d:%d:%s:%s:%s:%d:%d", _tenent, _company, window, _parameter1, _parameter2, tm.Hour(), tm.Minute())

	concEventNameWithoutParams := fmt.Sprintf("CONCURRENTWOPARAMS:%d:%d:%s", _tenent, _company, window)
	totCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:%d:%d:%s", _tenent, _company, window)
	concEventNameWithoutParams_businessUnit := fmt.Sprintf("CONCURRENTWOPARAMS:%d:%d:%s:%s", _tenent, _company, _businessUnit, window)
	totCountEventNameWithoutParams_businessUnit := fmt.Sprintf("TOTALCOUNTWOPARAMS:%d:%d:%s:%s", _tenent, _company, _businessUnit, window)

	concEventNameWithSingleParam := fmt.Sprintf("CONCURRENTWSPARAM:%d:%d:%s:%s", _tenent, _company, window, _parameter1)
	totCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:%d:%d:%s:%s", _tenent, _company, window, _parameter1)
	concEventNameWithSingleParam_businessUnit := fmt.Sprintf("CONCURRENTWSPARAM:%d:%d:%s:%s:%s", _tenent, _company, _businessUnit, window, _parameter1)
	totCountEventNameWithSingleParam_businessUnit := fmt.Sprintf("TOTALCOUNTWSPARAM:%d:%d:%s:%s:%s", _tenent, _company, _businessUnit, window, _parameter1)

	concEventNameWithLastParam := fmt.Sprintf("CONCURRENTWLPARAM:%d:%d:%s:%s", _tenent, _company, window, _parameter2)
	totCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:%d:%d:%s:%s", _tenent, _company, window, _parameter2)
	concEventNameWithLastParam_businessUnit := fmt.Sprintf("CONCURRENTWLPARAM:%d:%d:%s:%s:%s", _tenent, _company, _businessUnit, window, _parameter2)
	totCountEventNameWithLastParam_businessUnit := fmt.Sprintf("TOTALCOUNTWLPARAM:%d:%d:%s:%s:%s", _tenent, _company, _businessUnit, window, _parameter2)

	//--------------------------------statsD keys--------------------------
	countConcStatName := fmt.Sprintf("event.%s.concurrent.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, _businessUnit, _parameter1, window)
	gaugeConcStatName := fmt.Sprintf("event.%s.concurrent.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, _businessUnit, _parameter1, window)
	totCountStatName := fmt.Sprintf("event.%s.totalcount.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, _businessUnit, _parameter1, window)

	ccount, ccountErr := client.Cmd("incr", concEventName).Int()
	tcount, tcountErr := client.Cmd("incr", totCountEventName).Int()
	ccountB, ccountBErr := client.Cmd("incr", concEventName_businessUnit).Int()
	tcountB, tcountBErr := client.Cmd("incr", totCountEventName_businessUnit).Int()
	errHndlr("OnEvent", "Cmd ccountErr", ccountErr)
	errHndlr("OnEvent", "Cmd tcountErr", tcountErr)
	errHndlr("OnEvent", "Cmd tcountBErr", ccountBErr)
	errHndlr("OnEvent", "Cmd tcountBErr", tcountBErr)

	_, err1 := client.Cmd("incr", concEventNameWithoutParams).Int()
	_, err2 := client.Cmd("incr", totCountEventNameWithoutParams).Int()
	_, errB1 := client.Cmd("incr", concEventNameWithoutParams_businessUnit).Int()
	_, errB2 := client.Cmd("incr", totCountEventNameWithoutParams_businessUnit).Int()
	errHndlr("OnEvent", "Cmd err1", err1)
	errHndlr("OnEvent", "Cmd err2", err2)
	errHndlr("OnEvent", "Cmd errB1", errB1)
	errHndlr("OnEvent", "Cmd errB2", errB2)

	_, err3 := client.Cmd("incr", concEventNameWithSingleParam).Int()
	_, err4 := client.Cmd("incr", totCountEventNameWithSingleParam).Int()
	_, errB3 := client.Cmd("incr", concEventNameWithSingleParam_businessUnit).Int()
	_, errB4 := client.Cmd("incr", totCountEventNameWithSingleParam_businessUnit).Int()
	errHndlr("OnEvent", "Cmd err3", err3)
	errHndlr("OnEvent", "Cmd err4", err4)
	errHndlr("OnEvent", "Cmd errB3", errB3)
	errHndlr("OnEvent", "Cmd errB4", errB4)

	_, err5 := client.Cmd("incr", concEventNameWithLastParam).Int()
	_, err6 := client.Cmd("incr", totCountEventNameWithLastParam).Int()
	_, errB5 := client.Cmd("incr", concEventNameWithLastParam_businessUnit).Int()
	_, errB6 := client.Cmd("incr", totCountEventNameWithLastParam_businessUnit).Int()
	errHndlr("OnEvent", "Cmd err5", err5)
	errHndlr("OnEvent", "Cmd err6", err6)
	errHndlr("OnEvent", "Cmd errB5", errB5)
	errHndlr("OnEvent", "Cmd errB6", errB6)

	//errHndlr("OnEvent", "Cmd totCountHrEventName", client.Cmd("incr", totCountHrEventName).Err)

	fmt.Println("tcount ", tcount)
	fmt.Println("ccount ", ccount)
	fmt.Println("tcountB ", tcountB)
	fmt.Println("ccountB ", ccountB)

	statClient.Increment(countConcStatName)
	statClient.Gauge(gaugeConcStatName, ccountB)
	statClient.Gauge(totCountStatName, tcountB)

	DoPublish(_company, _tenent, _businessUnit, window, _parameter1, _parameter2)
	DoPublish(_company, _tenent, "*", window, _parameter1, _parameter2)
}

func DecrementEvent(_tenent, _company, tryCount int, window, _session, persistSession, statsDPath, threshold string, tm time.Time, location *time.Location, thresholdEnabled bool) {

	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in DecrementEvent", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnEvent", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnEvent", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	logDetails := fmt.Sprintf("Tenant: %d :: Company: %d :: TryCount: %d :: Window: %s :: Session: %s :: PersistSession: %s :: StatsDPath: %s :: threshold: %s :: TM: %s :: Location: %s :: ThresholdEnabled: %t", _tenent, _company, tryCount, window, _session, persistSession, statsDPath, threshold, tm.Format(layout), location.String(), thresholdEnabled)
	fmt.Println("DecrementEvent:: ", logDetails)

	sessionKey, timeValue, businessUnit, sParam1, sParam2 := FindDashboardSession(_tenent, _company, window, _session, persistSession)
	if sessionKey != "" {
		var timeDiff int
		var duration int64

		if timeValue != "" {
			tm2, _ := time.Parse(layout, timeValue)
			duration = int64(tm.Sub(tm2.In(location)) / time.Millisecond)
			timeDiff = int(tm.Sub(tm2.In(location)).Seconds())

			if timeDiff < 0 {
				timeDiff = 0
			}
		} else {
			timeDiff = 0
			duration = 0
		}

		fmt.Println(timeDiff)

		isdel := RemoveDashboardSession(_tenent, _company, window, _session, sessionKey, persistSession)
		if isdel == 1 {

			//---------------------Redis Keys-----------------------------------------------

			concEventName := fmt.Sprintf("CONCURRENT:%d:%d:%s:%s:%s", _tenent, _company, window, sParam1, sParam2)
			totTimeEventName := fmt.Sprintf("TOTALTIME:%d:%d:%s:%s:%s", _tenent, _company, window, sParam1, sParam2)

			maxTimeEventName := fmt.Sprintf("MAXTIME:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)
			thresholdEventName := fmt.Sprintf("THRESHOLD:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)
			thresholdBreakDownEventName := fmt.Sprintf("THRESHOLDBREAKDOWN:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)

			concEventNameWithoutParams := fmt.Sprintf("CONCURRENTWOPARAMS:%d:%d:%s", _tenent, _company, window)
			totTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:%d:%d:%s", _tenent, _company, window)

			concEventNameWithSingleParam := fmt.Sprintf("CONCURRENTWSPARAM:%d:%d:%s:%s", _tenent, _company, window, sParam1)
			totTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:%d:%d:%s:%s", _tenent, _company, window, sParam1)

			concEventNameWithLastParam := fmt.Sprintf("CONCURRENTWLPARAM:%d:%d:%s:%s", _tenent, _company, window, sParam2)
			totTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:%d:%d:%s:%s", _tenent, _company, window, sParam2)

			//---------------------Redis Keys Business Units-----------------------------------------------

			concEventName_BusinssUnit := fmt.Sprintf("CONCURRENT:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)
			totTimeEventName_BusinssUnit := fmt.Sprintf("TOTALTIME:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)
			//			maxTimeEventName_BusinssUnit := fmt.Sprintf("MAXTIME:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)
			//			thresholdEventName_BusinssUnit := fmt.Sprintf("THRESHOLD:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)
			//			thresholdBreakDownEventName_BusinssUnit := fmt.Sprintf("THRESHOLDBREAKDOWN:%d:%d:%s:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1, sParam2)

			concEventNameWithoutParams_BusinssUnit := fmt.Sprintf("CONCURRENTWOPARAMS:%d:%d:%s:%s", _tenent, _company, businessUnit, window)
			totTimeEventNameWithoutParams_BusinssUnit := fmt.Sprintf("TOTALTIMEWOPARAMS:%d:%d:%s:%s", _tenent, _company, businessUnit, window)

			concEventNameWithSingleParam_BusinssUnit := fmt.Sprintf("CONCURRENTWSPARAM:%d:%d:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1)
			totTimeEventNameWithSingleParam_BusinssUnit := fmt.Sprintf("TOTALTIMEWSPARAM:%d:%d:%s:%s:%s", _tenent, _company, businessUnit, window, sParam1)

			concEventNameWithLastParam_BusinssUnit := fmt.Sprintf("CONCURRENTWLPARAM:%d:%d:%s:%s:%s", _tenent, _company, businessUnit, window, sParam2)
			totTimeEventNameWithLastParam_BusinssUnit := fmt.Sprintf("TOTALTIMEWLPARAM:%d:%d:%s:%s:%s", _tenent, _company, businessUnit, window, sParam2)

			//---------------------StatsD Keys-----------------------------------------------

			countConcStatName := fmt.Sprintf("event.%s.concurrent.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, businessUnit, sParam1, window)
			gaugeConcStatName := fmt.Sprintf("event.%s.concurrent.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, businessUnit, sParam1, window)
			totTimeStatName := fmt.Sprintf("event.%s.totaltime.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, businessUnit, sParam1, window)
			timeStatName := fmt.Sprintf("event.%s.timer.%d.%d.%s.%s.%s", statsDPath, _tenent, _company, businessUnit, sParam1, window)

			_, rincErr := client.Cmd("incrby", totTimeEventName, timeDiff).Int()
			_, err2 := client.Cmd("incrby", totTimeEventNameWithoutParams, timeDiff).Int()
			_, err3 := client.Cmd("incrby", totTimeEventNameWithSingleParam, timeDiff).Int()
			_, err4 := client.Cmd("incrby", totTimeEventNameWithLastParam, timeDiff).Int()

			rincB, rincErrB := client.Cmd("incrby", totTimeEventName_BusinssUnit, timeDiff).Int()
			_, errB2 := client.Cmd("incrby", totTimeEventNameWithoutParams_BusinssUnit, timeDiff).Int()
			_, errB3 := client.Cmd("incrby", totTimeEventNameWithSingleParam_BusinssUnit, timeDiff).Int()
			_, errB4 := client.Cmd("incrby", totTimeEventNameWithLastParam_BusinssUnit, timeDiff).Int()

			dccount, dccountErr := client.Cmd("decr", concEventName).Int()
			_, err5 := client.Cmd("decr", concEventNameWithoutParams).Int()
			_, err6 := client.Cmd("decr", concEventNameWithSingleParam).Int()
			_, err7 := client.Cmd("decr", concEventNameWithLastParam).Int()

			dccountB, dccountErrB := client.Cmd("decr", concEventName_BusinssUnit).Int()
			_, errB5 := client.Cmd("decr", concEventNameWithoutParams_BusinssUnit).Int()
			_, errB6 := client.Cmd("decr", concEventNameWithSingleParam_BusinssUnit).Int()
			_, errB7 := client.Cmd("decr", concEventNameWithLastParam_BusinssUnit).Int()

			errHndlr("OnEvent", "Cmd rincErr", rincErr)
			errHndlr("OnEvent", "Cmd err2", err2)
			errHndlr("OnEvent", "Cmd err3", err3)
			errHndlr("OnEvent", "Cmd err4", err4)
			errHndlr("OnEvent", "Cmd dccountErr", dccountErr)
			errHndlr("OnEvent", "Cmd err5", err5)
			errHndlr("OnEvent", "Cmd err6", err6)
			errHndlr("OnEvent", "Cmd err7", err7)

			errHndlr("OnEvent", "Cmd rincErrB", rincErrB)
			errHndlr("OnEvent", "Cmd errB2", errB2)
			errHndlr("OnEvent", "Cmd errB3", errB3)
			errHndlr("OnEvent", "Cmd errB4", errB4)
			errHndlr("OnEvent", "Cmd dccountErrB", dccountErrB)
			errHndlr("OnEvent", "Cmd errB5", errB5)
			errHndlr("OnEvent", "Cmd errB6", errB6)
			errHndlr("OnEvent", "Cmd errB7", errB7)

			if dccount < 0 {
				fmt.Println("reset minus concurrent count:: incr by 1 :: ", concEventName)
				dccount, dccountErr = client.Cmd("incr", concEventName).Int()
				_, err8 := client.Cmd("incr", concEventNameWithoutParams).Int()
				_, err9 := client.Cmd("incr", concEventNameWithSingleParam).Int()
				_, err10 := client.Cmd("incr", concEventNameWithLastParam).Int()
				errHndlr("OnEvent", "Cmd dccountErr", dccountErr)
				errHndlr("OnEvent", "Cmd err8", err8)
				errHndlr("OnEvent", "Cmd err9", err9)
				errHndlr("OnEvent", "Cmd err10", err10)
			}
			if dccountB < 0 {
				fmt.Println("reset minus concurrent business unit count:: incr by 1 :: ", concEventName_BusinssUnit)
				dccountB, dccountErrB = client.Cmd("incr", concEventName_BusinssUnit).Int()
				_, errB8 := client.Cmd("incr", concEventNameWithoutParams_BusinssUnit).Int()
				_, errB9 := client.Cmd("incr", concEventNameWithSingleParam_BusinssUnit).Int()
				_, errB10 := client.Cmd("incr", concEventNameWithLastParam_BusinssUnit).Int()
				errHndlr("OnEvent", "Cmd dccountErrB", dccountErrB)
				errHndlr("OnEvent", "Cmd errB8", errB8)
				errHndlr("OnEvent", "Cmd errB9", errB9)
				errHndlr("OnEvent", "Cmd errB10", errB10)
			}

			oldMaxTime, oldMaxTimeErr := client.Cmd("get", maxTimeEventName).Int()
			errHndlr("OnEvent", "Cmd oldMaxTimeErr", oldMaxTimeErr)
			if oldMaxTime < timeDiff {
				errHndlr("OnEvent", "Cmd maxTimeEventName", client.Cmd("set", maxTimeEventName, timeDiff).Err)
			}
			if window != "QUEUE" {
				statClient.Decrement(countConcStatName)
			}
			if thresholdEnabled == true && threshold != "" {
				thValue, _ := strconv.Atoi(threshold)

				if thValue > 0 {
					thHour := tm.Hour()

					if timeDiff > thValue {
						thcount, thcountErr := client.Cmd("incr", thresholdEventName).Int()
						errHndlr("OnEvent", "Cmd thcountErr", thcountErr)
						fmt.Println(thresholdEventName, ": ", thcount)

						thValue_2 := thValue * 2
						thValue_4 := thValue * 4
						thValue_8 := thValue * 8
						thValue_10 := thValue * 10
						thValue_12 := thValue * 12

						fmt.Println("thValue_2::", thValue_2)
						fmt.Println("thValue_4::", thValue_4)
						fmt.Println("thValue_8::", thValue_8)
						fmt.Println("thValue_10::", thValue_10)
						fmt.Println("thValue_12::", thValue_12)

						if timeDiff > thValue && timeDiff <= thValue_2 {
							thresholdBreakDown_1 := fmt.Sprintf("%s:%d:%d:%d", thresholdBreakDownEventName, thHour, thValue, thValue_2)
							errHndlr("OnEvent", "Cmd thresholdBreakDown_1", client.Cmd("incr", thresholdBreakDown_1).Err)
							fmt.Println("thresholdBreakDown_1::", thresholdBreakDown_1)
						} else if timeDiff > thValue_2 && timeDiff <= thValue_4 {
							thresholdBreakDown_2 := fmt.Sprintf("%s:%d:%d:%d", thresholdBreakDownEventName, thHour, thValue_2, thValue_4)
							errHndlr("OnEvent", "Cmd thresholdBreakDown_2", client.Cmd("incr", thresholdBreakDown_2).Err)
							fmt.Println("thresholdBreakDown_2::", thresholdBreakDown_2)
						} else if timeDiff > thValue_4 && timeDiff <= thValue_8 {
							thresholdBreakDown_3 := fmt.Sprintf("%s:%d:%d:%d", thresholdBreakDownEventName, thHour, thValue_4, thValue_8)
							errHndlr("OnEvent", "Cmd thresholdBreakDown_3", client.Cmd("incr", thresholdBreakDown_3).Err)
							fmt.Println("thresholdBreakDown_3::", thresholdBreakDown_3)
						} else if timeDiff > thValue_8 && timeDiff <= thValue_10 {
							thresholdBreakDown_4 := fmt.Sprintf("%s:%d:%d:%d", thresholdBreakDownEventName, thHour, thValue_8, thValue_10)
							errHndlr("OnEvent", "Cmd thresholdBreakDown_4", client.Cmd("incr", thresholdBreakDown_4).Err)
							fmt.Println("thresholdBreakDown_4::", thresholdBreakDown_4)
						} else if timeDiff > thValue_10 && timeDiff <= thValue_12 {
							thresholdBreakDown_5 := fmt.Sprintf("%s:%d:%d:%d", thresholdBreakDownEventName, thHour, thValue_10, thValue_12)
							errHndlr("OnEvent", "Cmd thresholdBreakDown_5", client.Cmd("incr", thresholdBreakDown_5).Err)
							fmt.Println("thresholdBreakDown_5::", thresholdBreakDown_5)
						} else {
							thresholdBreakDown_6 := fmt.Sprintf("%s:%d:%d:%s", thresholdBreakDownEventName, thHour, thValue_12, "gt")
							errHndlr("OnEvent", "Cmd thresholdBreakDown_6", client.Cmd("incr", thresholdBreakDown_6).Err)
							fmt.Println("thresholdBreakDown_6::", thresholdBreakDown_6)
						}
					} else {
						thresholdBreakDown_7 := fmt.Sprintf("%s:%d:%s:%d", thresholdBreakDownEventName, thHour, "lt", thValue)
						errHndlr("OnEvent", "Cmd thresholdBreakDown_7", client.Cmd("incr", thresholdBreakDown_7).Err)
						fmt.Println("thresholdBreakDown_7::", thresholdBreakDown_7)
					}
				}
			}
			statClient.Gauge(gaugeConcStatName, dccountB)
			statClient.Gauge(totTimeStatName, rincB)

			statClient.Timing(timeStatName, duration)

			DoPublish(_company, _tenent, businessUnit, window, sParam1, sParam2)
			DoPublish(_company, _tenent, "*", window, sParam1, sParam2)
		} else {
			fmt.Println("Delete session: ", _session, " failed")
		}
	} else {
		fmt.Println("Session data not found for decriment: ", _session, " :: tryCount: ", tryCount)

		decrRetryCountInt, _ := strconv.Atoi(decrRetryCount)

		if tryCount < decrRetryCountInt {
			var reTryDetail = DecrRetryDetail{}
			reTryDetail.Company = _company
			reTryDetail.Tenant = _tenent
			reTryDetail.Window = window
			reTryDetail.Session = _session
			reTryDetail.PersistSession = persistSession
			reTryDetail.StatsDPath = statsDPath
			reTryDetail.Threshold = threshold
			reTryDetail.EventTime = tm.Format(layout)
			reTryDetail.ExecutionTime = time.Now().In(location).Format(layout)
			reTryDetail.TimeLocation = location.String()
			reTryDetail.ThresholdEnabled = thresholdEnabled
			reTryDetail.TryCount = tryCount

			reTryDetailMarshalData, mErr := json.Marshal(reTryDetail)
			if mErr != nil {
				fmt.Println("Marshal Retry data failed: ", _session, " :: Error: ", mErr.Error())
			} else {
				reTryDetailJsonString := string(reTryDetailMarshalData)
				_, lpushErr := client.Cmd("hset", "DecrRetrySessions", _session, reTryDetailJsonString).Int()
				if lpushErr != nil {
					fmt.Println("Lpush retry data failed: ", _session, " :: Error: ", lpushErr.Error())
				}
			}
		}
	}
}

func ProcessDecrRetry() {
	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in ProcessDecrRetry", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnEvent", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnEvent", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	decrEvents, _ := client.Cmd("hgetall", "DecrRetrySessions").Map()
	for _, event := range decrEvents {
		var decrEventDetail DecrRetryDetail
		json.Unmarshal([]byte(event), &decrEventDetail)

		location, _ := time.LoadLocation(decrEventDetail.TimeLocation)

		tm := time.Now().In(location)

		tm2, _ := time.Parse(layout, decrEventDetail.EventTime)
		tm3, _ := time.Parse(layout, decrEventDetail.ExecutionTime)
		eventTime := tm2.In(location)
		executionTime := tm3.In(location)
		timeDiff := int(tm.Sub(executionTime).Seconds())

		decrRetryDelayInt, _ := strconv.Atoi(decrRetryDelay)
		if timeDiff >= decrRetryDelayInt {

			fmt.Println("Execute decr late event session: ", decrEventDetail.Session)
			decrEventDetail.TryCount++

			client.Cmd("hdel", "DecrRetrySessions", decrEventDetail.Session)
			DecrementEvent(decrEventDetail.Tenant, decrEventDetail.Company, decrEventDetail.TryCount, decrEventDetail.Window, decrEventDetail.Session, decrEventDetail.PersistSession, decrEventDetail.StatsDPath, decrEventDetail.Threshold, eventTime, location, decrEventDetail.ThresholdEnabled)

		} else {
			fmt.Println("Execute decr late event session: ", decrEventDetail.Session, " :: Waiting")
		}
	}
}

func OnReset() {

	_searchName := fmt.Sprintf("META:*:FLUSH")
	fmt.Println("Search Windows to Flush: ", _searchName)

	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnReset", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnReset", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnReset", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	//client, err := redis.DialTimeout("tcp", redisIp, time.Duration(10)*time.Second)
	//errHndlr(err)
	//defer client.Close()

	/*//authServer
	authE := client.Cmd("auth", redisPassword)
	errHndlr("auth", authE.Err)
	// select database
	r := client.Cmd("select", redisDb)
	errHndlr("selectDb", r.Err)*/

	_windowList := make([]string, 0)
	_keysToRemove := make([]string, 0)
	_loginSessions := make([]string, 0)
	_productivitySessions := make([]string, 0)

	if cacheMachenism == "redis" {

		val := ScanAndGetKeys(_searchName)
		lenth := len(val)
		fmt.Println(lenth)
		if lenth > 0 {
			for _, value := range val {
				tmx, tmxErr := client.Cmd("get", value).Str()
				errHndlr("OnReset", "Cmd", tmxErr)

				_windowList = AppendIfMissing(_windowList, tmx)
			}

		}

	} else {
		fmt.Println("---------------------Use Memoey----------------------")
		for _, dmi := range dashboardMetaInfo {
			if dmi.FlushEnable == true {
				_windowList = AppendIfMissing(_windowList, dmi.WindowName)
			}
		}

		fmt.Println("Windoes To Flush:: ", _windowList)
	}

	for _, window := range _windowList {

		fmt.Println("WindowList_: ", window)

		//snapEventSearch := fmt.Sprintf("SNAPSHOT:*:%s:*", window)
		//snapHourlyEventSearch := fmt.Sprintf("SNAPSHOTHOURLY:*:%s:*", window)
		concEventSearch := fmt.Sprintf("CONCURRENT:*:%s:*", window)
		sessEventSearch := fmt.Sprintf("SESSION:*:%s:*", window)
		sessParamsEventSearch := fmt.Sprintf("SESSIONPARAMS:*:%s:*", window)
		totTimeEventSearch := fmt.Sprintf("TOTALTIME:*:%s:*", window)
		totCountEventSearch := fmt.Sprintf("TOTALCOUNT:*:%s:*", window)
		totCountHr := fmt.Sprintf("TOTALCOUNTHR:*:%s:*", window)
		maxTimeEventSearch := fmt.Sprintf("MAXTIME:*:%s:*", window)
		thresholdEventSearch := fmt.Sprintf("THRESHOLD:*:%s:*", window)
		thresholdBDEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:*:%s:*", window)

		concEventNameWithoutParams := fmt.Sprintf("CONCURRENTWOPARAMS:*:%s", window)
		totTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:*:%s", window)
		totCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:*:%s", window)

		concEventNameWithSingleParam := fmt.Sprintf("CONCURRENTWSPARAM:*:%s:*", window)
		totTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:*:%s:*", window)
		totCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:*:%s:*", window)

		concEventNameWithLastParam := fmt.Sprintf("CONCURRENTWLPARAM:*:%s:*", window)
		totTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:*:%s:*", window)
		totCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:*:%s:*", window)

		//snapVal, _ := client.Cmd("keys", snapEventSearch).List()
		//_keysToRemove = AppendListIfMissing(_keysToRemove, snapVal)

		//snapHourlyVal, _ := client.Cmd("keys", snapHourlyEventSearch).List()
		//_keysToRemove = AppendListIfMissing(_keysToRemove, snapHourlyVal)

		concVal := ScanAndGetKeys(concEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, concVal)

		sessParamsVal := ScanAndGetKeys(sessParamsEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, sessParamsVal)

		sessVal := ScanAndGetKeys(sessEventSearch)
		for _, sess := range sessVal {
			sessItems := strings.Split(sess, ":")
			if len(sessItems) >= 5 && (sessItems[4] == "LOGIN" || sessItems[4] == "INBOUND" || sessItems[4] == "OUTBOUND") {
				_loginSessions = AppendIfMissing(_loginSessions, sess)
			} else if len(sessItems) >= 4 && sessItems[4] == "PRODUCTIVITY" {
				_productivitySessions = AppendIfMissing(_productivitySessions, sess)
			} else {
				_keysToRemove = AppendIfMissing(_keysToRemove, sess)
			}
		}

		totTimeVal := ScanAndGetKeys(totTimeEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totTimeVal)

		totCountVal := ScanAndGetKeys(totCountEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountVal)

		totCountHrVal := ScanAndGetKeys(totCountHr)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountHrVal)

		maxTimeVal := ScanAndGetKeys(maxTimeEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, maxTimeVal)

		thresholdCountVal := ScanAndGetKeys(thresholdEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdCountVal)

		thresholdBDCountVal := ScanAndGetKeys(thresholdBDEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdBDCountVal)

		cewop := ScanAndGetKeys(concEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewop)

		ttwop := ScanAndGetKeys(totTimeEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwop)

		tcewop := ScanAndGetKeys(totCountEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcewop)

		cewsp := ScanAndGetKeys(concEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewsp)

		ttwsp := ScanAndGetKeys(totTimeEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwsp)

		tcwsp := ScanAndGetKeys(totCountEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwsp)

		cewlp := ScanAndGetKeys(concEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewlp)

		ttwlp := ScanAndGetKeys(totTimeEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwlp)

		tcwlp := ScanAndGetKeys(totCountEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwlp)

	}
	tm := time.Now()
	for _, remove := range _keysToRemove {
		fmt.Println("remove_: ", remove)
		errHndlr("OnReset", "Cmd", client.Cmd("del", remove).Err)
	}
	for _, session := range _loginSessions {
		fmt.Println("readdSession: ", session)
		errHndlr("OnReset", "Cmd", client.Cmd("hset", session, "time", tm.Format(layout)).Err)
		sessItemsL := strings.Split(session, ":")

		if len(sessItemsL) >= 7 {
			LsessParamEventName := fmt.Sprintf("SESSIONPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[5])
			LtotTimeEventName := fmt.Sprintf("TOTALTIME:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotCountEventName := fmt.Sprintf("TOTALCOUNT:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4])
			LtotCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4])
			LtotTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6])
			LtotCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6])
			LtotTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[7])
			LtotCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[7])

			LtotTimeEventName_BusinessUnit := fmt.Sprintf("TOTALTIME:%s:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotCountEventName_BusinessUnit := fmt.Sprintf("TOTALCOUNT:%s:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotTimeEventNameWithoutParams_BusinessUnit := fmt.Sprintf("TOTALTIMEWOPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4])
			LtotCountEventNameWithoutParams_BusinessUnit := fmt.Sprintf("TOTALCOUNTWOPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4])
			LtotTimeEventNameWithSingleParam_BusinessUnit := fmt.Sprintf("TOTALTIMEWSPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6])
			LtotCountEventNameWithSingleParam_BusinessUnit := fmt.Sprintf("TOTALCOUNTWSPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6])
			LtotTimeEventNameWithLastParam_BusinessUnit := fmt.Sprintf("TOTALTIMEWLPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[7])
			LtotCountEventNameWithLastParam_BusinessUnit := fmt.Sprintf("TOTALCOUNTWLPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[7])

			errHndlr("OnReset", "Cmd", client.Cmd("hmset", LsessParamEventName, "businessUnit", sessItemsL[3], "param1", sessItemsL[6], "param2", sessItemsL[7]).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventName, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventName, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithoutParams, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithoutParams, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithSingleParam, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithSingleParam, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithLastParam, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithLastParam, 0).Err)

			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventName_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventName_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithoutParams_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithoutParams_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithSingleParam_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithSingleParam_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithLastParam_BusinessUnit, 0).Err)
			errHndlr("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithLastParam_BusinessUnit, 0).Err)
		}
	}
	/*for _, prosession := range _productivitySessions {
		fmt.Println("readdSession: ", prosession)
		client.Cmd("hset", prosession, "time", tm.Format(layout))
	}*/
}

func OnSetDailySummary(_date time.Time) {

	var client *redis.Client
	var err error

	totCountEventSearch := fmt.Sprintf("TOTALCOUNT:*")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnReset", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnSetDailySummary", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnSetDailySummary", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	//client, err := redis.DialTimeout("tcp", redisIp, time.Duration(10)*time.Second)
	//errHndlr(err)
	//defer client.Close()

	/*//authServer
	authE := client.Cmd("auth", redisPassword)
	errHndlr("auth", authE.Err)
	// select database
	r := client.Cmd("select", redisDb)
	errHndlr("selectDb", r.Err)*/

	totalEventKeys := ScanAndGetKeys(totCountEventSearch)
	for _, key := range totalEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 7 {
			summery := SummeryDetail{}
			tenant, _ := strconv.Atoi(keyItems[1])
			company, _ := strconv.Atoi(keyItems[2])
			summery.Tenant = tenant
			summery.Company = company
			summery.BusinessUnit = keyItems[3]
			summery.WindowName = keyItems[4]
			summery.Param1 = keyItems[5]
			summery.Param2 = keyItems[6]

			currentTime := 0
			if summery.WindowName == "LOGIN" {
				sessEventSearch := fmt.Sprintf("SESSION:%d:%d:%s:%s:*:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
				sessEvents := ScanAndGetKeys(sessEventSearch)
				if len(sessEvents) > 0 {
					tmx, tmxErr := client.Cmd("hget", sessEvents[0], "time").Str()
					errHndlr("OnSetDailySummary", "Cmd", tmxErr)
					if tmx != "" {
						tm2, _ := time.Parse(layout, tmx)
						currentTime = int(_date.Sub(tm2.Local()).Seconds())
						fmt.Println("currentTime: ", currentTime)
					}
				}
			}
			totTimeEventName := fmt.Sprintf("TOTALTIME:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
			maxTimeEventName := fmt.Sprintf("MAXTIME:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
			thresholdEventName := fmt.Sprintf("THRESHOLD:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)

			fmt.Println("totTimeEventNameBU: ", totTimeEventName)
			fmt.Println("maxTimeEventNameBU: ", maxTimeEventName)
			fmt.Println("thresholdEventNameBU: ", thresholdEventName)

			totCount, totCountErr := client.Cmd("get", key).Int()
			totTime, totTimeErr := client.Cmd("get", totTimeEventName).Int()
			maxTime, maxTimeErr := client.Cmd("get", maxTimeEventName).Int()
			threshold, thresholdErr := client.Cmd("get", thresholdEventName).Int()

			errHndlr("OnSetDailySummary", "Cmd", totCountErr)
			errHndlr("OnSetDailySummary", "Cmd", totTimeErr)
			errHndlr("OnSetDailySummary", "Cmd", maxTimeErr)
			errHndlr("OnSetDailySummary", "Cmd", thresholdErr)

			fmt.Println("totCount: ", totCount)
			fmt.Println("totTime: ", totTime)
			fmt.Println("maxTime: ", maxTime)
			fmt.Println("threshold: ", threshold)

			summery.TotalCount = totCount
			summery.TotalTime = totTime + currentTime
			summery.MaxTime = maxTime
			summery.ThresholdValue = threshold
			summery.SummaryDate = _date
			go PersistsSummaryData(summery)
		}
	}
}

func OnSetDailyThesholdBreakDown(_date time.Time) {

	var client *redis.Client
	var err error

	thresholdEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:*")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailyThesholdBreakDown", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("OnSetDailyThesholdBreakDown", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHndlr("OnSetDailyThesholdBreakDown", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	//client, err := redis.DialTimeout("tcp", redisIp, time.Duration(10)*time.Second)
	//errHndlr(err)
	//defer client.Close()

	/*//authServer
	authE := client.Cmd("auth", redisPassword)
	errHndlr("auth", authE.Err)
	// select database
	r := client.Cmd("select", redisDb)
	errHndlr("selectDb", r.Err)*/

	thresholdEventKeys := ScanAndGetKeys(thresholdEventSearch)
	for _, key := range thresholdEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 10 {
			summery := ThresholdBreakDownDetail{}
			tenant, _ := strconv.Atoi(keyItems[1])
			company, _ := strconv.Atoi(keyItems[2])
			hour, _ := strconv.Atoi(keyItems[7])
			summery.Tenant = tenant
			summery.Company = company
			summery.BusinessUnit = keyItems[3]
			summery.WindowName = keyItems[4]
			summery.Param1 = keyItems[5]
			summery.Param2 = keyItems[6]
			summery.BreakDown = fmt.Sprintf("%s-%s", keyItems[8], keyItems[9])
			summery.Hour = hour

			thCount, thCountErr := client.Cmd("get", key).Int()
			errHndlr("OnSetDailyThesholdBreakDown", "Cmd", thCountErr)
			summery.ThresholdCount = thCount
			summery.SummaryDate = _date

			go PersistsThresholdBreakDown(summery)
		}
	}
}

func AppendIfMissing(windowList []string, i string) []string {
	for _, ele := range windowList {
		if ele == i {
			return windowList
		}
	}
	return append(windowList, i)
}

func AppendListIfMissing(windowList1 []string, windowList2 []string) []string {
	notExist := true
	for _, ele2 := range windowList2 {
		for _, ele := range windowList1 {
			if ele == ele2 {
				notExist = false
				break
			}
		}

		if notExist == true {
			windowList1 = append(windowList1, ele2)
		}
	}

	return windowList1
}

func FindDashboardSession(_tenant, _company int, _window, _session, _persistSession string) (sessionKey, timeValue, businessUnit, param1, param2 string) {

	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in FindDashboardSession", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if _persistSession == "true" {
		sessionKey, timeValue, businessUnit, param1, param2 = FindPersistedSession(_tenant, _company, _window, _session)
		return
	} else {

		if redisMode == "sentinel" {
			client, err = sentinelPool.GetMaster(redisClusterName)
			errHndlr("FindDashboardSession", "getConnFromSentinel", err)
		} else {
			client, err = redisPool.Get()
			errHndlr("FindDashboardSession", "getConnFromPool", err)
		}

		sessParamsEventKey := fmt.Sprintf("SESSIONPARAMS:%d:%d:%s:%s", _tenant, _company, _window, _session)

		isExists, isExistErr := client.Cmd("exists", sessParamsEventKey).Int()
		errHndlr("FindDashboardSession", "exists", isExistErr)

		if isExists == 1 {
			paramList, paramListErr := client.Cmd("hmget", sessParamsEventKey, "businessUnit", "param1", "param2").List()
			errHndlr("FindDashboardSession", "Cmd", paramListErr)
			if len(paramList) >= 3 {
				sessionKey = fmt.Sprintf("SESSION:%d:%d:%s:%s:%s:%s:%s", _tenant, _company, paramList[0], _window, _session, paramList[1], paramList[2])
				tmx, tmxErr := client.Cmd("hget", sessionKey, "time").Str()
				errHndlr("FindDashboardSession", "Cmd", tmxErr)
				timeValue = tmx
				businessUnit = paramList[0]
				param1 = paramList[1]
				param2 = paramList[2]

				errHndlr("FindDashboardSession", "Cmd", client.Cmd("del", sessParamsEventKey).Err)
			}
		}

		return
	}
}

func RemoveDashboardSession(_tenant, _company int, _window, _session, sessionKey, _persistSession string) (result int) {
	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in FindDashboardSession", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if _persistSession == "true" {
		result = DeletePersistedSession(_tenant, _company, _window, _session)
		return
	} else {

		if redisMode == "sentinel" {
			client, err = sentinelPool.GetMaster(redisClusterName)
			errHndlr("RemoveDashboardSession", "getConnFromSentinel", err)
			//defer sentinelPool.PutMaster(redisClusterName, client)
		} else {
			client, err = redisPool.Get()
			errHndlr("RemoveDashboardSession", "getConnFromPool", err)
			//defer redisPool.Put(client)
		}

		//client, err := redis.DialTimeout("tcp", redisIp, time.Duration(10)*time.Second)
		//errHndlr(err)
		//defer client.Close()

		/*//authServer
		authE := client.Cmd("auth", redisPassword)
		errHndlr("auth", authE.Err)
		// select database
		r := client.Cmd("select", redisDb)
		errHndlr("selectDb", r.Err)*/

		iDel, iDelErr := client.Cmd("del", sessionKey).Int()
		errHndlr("RemoveDashboardSession", "Cmd", iDelErr)
		result = iDel
		return
	}
}

func DoPublish(company, tenant int, businessUnit, window, param1, param2 string) {
	authToken := fmt.Sprintf("Bearer %s", accessToken)
	internalAuthToken := fmt.Sprintf("%d:%d", tenant, company)
	serviceurl := fmt.Sprintf("http://%s/DashboardEvent/Publish/%s/%s/%s/%s", CreateHost(dashboardServiceHost, dashboardServicePort), businessUnit, window, param1, param2)
	fmt.Println("URL:>", serviceurl)

	var jsonData = []byte("")
	req, err := http.NewRequest("POST", serviceurl, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", authToken)
	req.Header.Set("companyinfo", internalAuthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		//panic(err)
		//return false
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	//body, _ := ioutil.ReadAll(resp.Body)
	//result := string(body)
	fmt.Println("response CODE::", string(resp.StatusCode))
	fmt.Println("End======================================:: ", time.Now().UTC())
	if resp.StatusCode == 200 {
		fmt.Println("Return true")
		//return true
	}

	fmt.Println("Return false")
	//return false
}

func ScanAndGetKeys(pattern string) []string {
	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in ScanAndGetKeys", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	matchingKeys := make([]string, 0)

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHndlr("ScanAndGetKeys", "getConnFromSentinel", err)
	} else {
		client, err = redisPool.Get()
		errHndlr("ScanAndGetKeys", "getConnFromPool", err)
	}

	fmt.Println("Start ScanAndGetKeys:: ", pattern)
	scanResult := util.NewScanner(client, util.ScanOpts{Command: "SCAN", Pattern: pattern, Count: 1000})

	for scanResult.HasNext() {
		matchingKeys = AppendIfMissing(matchingKeys, scanResult.Next())
	}
	fmt.Println("Scan Result:: ", matchingKeys)
	return matchingKeys
}

func CreateHost(_ip, _port string) string {
	testIp := net.ParseIP(_ip)
	if testIp.To4() == nil {
		return _ip
	} else {
		return fmt.Sprintf("%s:%s", _ip, _port)
	}
}
