//  Vitata
//
//  @file     worker.go
//  @author   cf <feng.chen@duobei.com>
//  @date     Wed Apr 23 10:12:00 2014
//  @version  golang 1.1.2 redis 2.6.4
//  @brief    Rewrite the worker.lua in luajit 2.0.2 and lua 5.1
//
//  http://www.duobei.com
//  http://www.wangxiaotong.com
//  http://www.jiangzuotong.com
//

package main

import (
	"encoding/json"
	"fmt"
	simplejson "github.com/bitly/go-simplejson"
	l4g "github.com/cfxks1989/log4go"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
	common "worker/common"
	vitata_status "worker/vitata_status"
	worker_net "worker/worker_net"
)

const (
	INSTANCE_ALIVE_TIMEOUT        = 60e9 // 1 min unit: nano
	CHECK_INSTANCE_ALIVE_INTERVAL = 10e9 // 10 seconds unit: nano
	VERSION                       = "1.0.1"
)

/*
	状态中心 01:
	proxyRouterAddr : "ipc:///tmp/proxy-appsd-1.ipc"

	redisServerPort : 6381
	workerCode      :  worker 标识码
	intervalTime    : the interval time of worker to count the number of message.
	worker 进程 001
	{
    	"redisServerAddress": "127.0.0.1",
    	"redisServerPort": "6381",
    	"proxyRouterAddr": "ipc:///tmp/proxy-appsd-1.ipc",
    	"logFilePath" : "/var/duobei/logs/vttworker-apps",
    	"sndhwmValue" : 1000000,
    	"rcvhwmValue" : 1000000,
    	"intervalTime" : 60000,
    	"workerCode": "01-001",
    	"loglevel": "INFO"
	}

 	worker 进程 002
	{
    	"redisServerAddress": "127.0.0.1",
    	"redisServerPort": "6381",
    	"proxyRouterAddr": "ipc:///tmp/proxy-appsd-1.ipc",
    	"logFilePath" : "/var/duobei/logs/vttworker-apps",
    	"sndhwmValue" : 1000000,
    	"rcvhwmValue" : 1000000,
    	"intervalTime" : 60000,
    	"workerCode": "01-001",
    	"loglevel": "INFO"
	}

	------------------------------------------------------------------------------------------

	状态中心 02:
	proxyRouterAddr : "ipc:///tmp/proxy-appsd-2.ipc"

	redisServerPort : 6382
	workerCode      :  worker 标识码

     worker 进程 001
	{
    	"redisServerAddress": "127.0.0.1",
    	"redisServerPort": "6382",
    	"proxyRouterAddr": "ipc:///tmp/proxy-appsd-2.ipc",
    	"logFilePath" : "/var/duobei/logs/vttworker-apps",
    	"sndhwmValue" : 1000000,
    	"rcvhwmValue" : 1000000,
    	"intervalTime" : 60000,
    	"workerCode": "02-001",
    	"loglevel": "INFO"
	}


 	 worker 进程 002
	{
    	"redisServerAddress": "127.0.0.1",
    	"redisServerPort": "6382",
    	"proxyRouterAddr": "ipc:///tmp/proxy-appsd-2.ipc",
    	"logFilePath" : "/var/duobei/logs/vttworker-apps",
    	"sndhwmValue" : 1000000,
    	"rcvhwmValue" : 1000000,
    	"intervalTime" : 60000,
    	"workerCode": "02-001",
    	"loglevel": "INFO"
	}
*/

type Configure struct {
	logFilePath        string
	loglevel           string
	redisServerAddress string
	redisServerPort    string
	proxyRouterAddr    string
	sndhwmValue        int
	rcvhwmValue        int
	intervalTime       int
	workerCode         string
	workerId           string // "WORKER:" + proxyRouterAddr + ":" + workerCode
}

var configuration Configure
var aliveInstances map[string]bool
var workerId string

var curTimestamp time.Time
var lastTimestamp time.Time
var logger l4g.Logger
var workerNet *worker_net.WorkerNet

/*
params		:input interface{}.
return		:result string.
description	:change the input into json String.
*/
func serialize(input interface{}) (result string) {
	resultByte, _ := json.Marshal(input)
	result = string(resultByte)
	return
}

var vitataStatus *vitata_status.VitataStatus

/*
	收到服务器进程的PING消息
	更新aliveInfo的时间戳
	instanceID: $instanceIP:$instancePID
	example: instanceID = "210.73.212.121:1089"
*/
func onPingMsg(instanceID string) {
	aliveInstances[instanceID] = true
	vitataStatus.SetInstanceAliveInfo(instanceID, workerId)
}

/*
	收到服务器进程销毁房间时候发出的UNREGAPP消息
*/
func onUnRegisterAppMsg(instanceIP string, instancePID string, appName string) {
	vitataStatus.RemoveUsersFromAppOfInstance(instanceIP, instancePID, appName, false)
}

/*
	connectMsg:当房间的第一用户上线会产生。XPUB的收到的订阅消息。
*/
func onConnectMsg(appName string, isConnect bool) {
	//if isConnect {
	//	logger.Info("%s connect.", appName)
	//} else {
	//	logger.Info("%s disconnect.", appName)
	//}
}

/*
	PUB:UPDATE消息
*/
func onBroadcastEventMsg(eventMsg common.CustomMap) common.CustomMap {
	logger.Debug("onBroadcastEventMsg")
	return vitataStatus.AppsProcessUpdateEvent(eventMsg)
}

/*
  REQ:INIT:教室初始化请求消息
*/

func onRequestInitMsg(sockIdentity string, cliIdentity string, appName string) common.CustomMap {
	logger.Debug("onRequestInitMsg [ %s ] [ %s ] [ %s ]", sockIdentity, cliIdentity, appName)
	return vitataStatus.AppsGetGlobalRoomStatus(sockIdentity, cliIdentity, appName)
}

/*
  REQ:EVENT：
  1、SnapshotRequestEvent；
  2、FullPingEvent；
  3、SendToClientEvent
*/

func onRequestEventMsg(sockIdentity string, cliIdentity string, eventMsg common.CustomMap) common.CustomMap {
	logger.Debug("onRequestEventMsg [ %s ] [ %s ]", sockIdentity, cliIdentity)
	if eventMsg == nil {
		return nil
	}
	var appName string
	event, ok := eventMsg["event"].(map[string]interface{})
	if ok {
		appName, _ = eventMsg["appName"].(string)
	}

	var eventToClient common.CustomMap
	if event["type"] == "SnapshotRequestEvent" {
		logger.Info("SnapshotRequestEvent: %s.", serialize(eventMsg))

		startVersion, _ := event["startVersion"].(json.Number).Int64()
		endVersion, _ := event["endVersion"].(json.Number).Int64()
		if startVersion < 1 {
			startVersion = 1
		}
		if endVersion < 1 {
			endVersion = 1
		}
		updateEventList := vitataStatus.AppsGetUpdateEventList(sockIdentity, cliIdentity, appName,
			startVersion, endVersion)

		snapshotRequestEventToClient := make(common.CustomMap)
		_event := make(common.CustomMap)
		_event["type"] = "SnapshotRequestEvent"
		_event["updateEventList"] = updateEventList
		snapshotRequestEventToClient["event"] = _event
		snapshotRequestEventToClient["appName"] = appName
		eventToClient = snapshotRequestEventToClient
	} else if event["type"] == "FullPingEvent" {
		eventToClient = eventMsg
	} else if event["type"] == "SendToClientEvent" {
		//TODO 有可能消息并没有发送出去
		vitataStatus.AppsProcessSendToClientEvent(eventMsg)
		replyEventToClient := make(common.CustomMap)
		_event := make(common.CustomMap)
		_event["isReply"] = true
		_event["type"] = "SendToClientEvent"
		replyEventToClient["event"] = _event
		replyEventToClient["appName"] = appName
		eventToClient = replyEventToClient
	} else {
		logger.Warn("req event type is dismatch.")
	}
	return eventToClient
}

//vitata_status notify worker_net callback
func sendRequestEventToClient(sockIdentity string, cliIdentity string, eventMsg common.CustomMap) {
	//logger.Debug("sendRequestEventToClient: %s | %s | %s ", sockIdentity, cliIdentity, serialize(eventMsg))
	workerNet.SendReplyEventMessage(sockIdentity, cliIdentity, eventMsg)
}

func sendBroadcastRequestToClient(sockIdentity string, cliIdentity string, eventMsg common.CustomMap) {
	//logger.Debug("sendBroadcastRequestToClient: %s | %s | %s ", sockIdentity, cliIdentity, serialize(eventMsg))
	workerNet.SendBroadcastMessage(sockIdentity, cliIdentity, eventMsg)
}

func checkInstanceAlive() {
	//var clearList []string
	for instanceId, isAlive := range aliveInstances {
		aliveInfo := vitataStatus.GetInstanceAliveInfoByInstanceId(instanceId, workerId)
		if aliveInfo.WorkerId == workerId && isAlive {
			if curTimestamp.Sub(aliveInfo.Timestamp).Nanoseconds() > INSTANCE_ALIVE_TIMEOUT {
				logger.Warn("instanceId [%s]: aliveInfo[%v] is timeout, curTimestamp is %v",
					instanceId, aliveInfo, curTimestamp)
				aliveInstances[instanceId] = false
				//vlist := strings.Split(instanceId, ":")
				//vitataStatus.RemoveInstance(vlist[0], vlist[1], workerId)
				//clearList = append(clearList, instanceId)
			}
		} /* else {
			logger.Debug("instanceId: %s is dead.", instanceId)
			clearList = append(clearList, instanceId)
		}*/
	}
	//// clear invalide instanceId
	//for _, instanceId := range clearList {
	//	delete(aliveInstances, instanceId)
	//}
}

func init() {
	var configFilename string
	if len(os.Args) == 2 {
		configFilename = os.Args[1]
	} else {
		fmt.Printf("CMD eg: vttworker config.json\n")
		os.Exit(1)
	}
	configContent, err := ioutil.ReadFile(configFilename)
	if err != nil {
		fmt.Printf("vttworker read config file error: %v", err)
		os.Exit(1)
	}

	configurationJson, jerr := simplejson.NewJson(configContent)
	if jerr != nil {
		fmt.Println("vttworker config json read error: %v", jerr)
		os.Exit(1)
	}

	configuration.proxyRouterAddr = configurationJson.Get("proxyRouterAddr").MustString()
	configuration.sndhwmValue = configurationJson.Get("sndhwmValue").MustInt()
	configuration.rcvhwmValue = configurationJson.Get("rcvhwmValue").MustInt()
	configuration.intervalTime = configurationJson.Get("intervalTime").MustInt()
	configuration.workerCode = configurationJson.Get("workerCode").MustString()

	configuration.redisServerAddress = configurationJson.Get("redisServerAddress").MustString()
	configuration.redisServerPort = configurationJson.Get("redisServerPort").MustString()
	configuration.workerId = strings.Join([]string{"WORKER:", configuration.proxyRouterAddr, ":", configuration.workerCode}, "")
	configuration.logFilePath = configurationJson.Get("logFilePath").MustString()
	configuration.loglevel = configurationJson.Get("loglevel").MustString()

	workerId = configuration.workerId
	loglevelMap := map[string]l4g.Level{
		"DEBUG":   l4g.DEBUG,
		"TRACE":   l4g.TRACE,
		"INFO":    l4g.INFO,
		"WARNING": l4g.WARNING,
		"ERROR":   l4g.ERROR,
	}
	logFileName := path.Join(configuration.logFilePath, "vttworker-apps-"+configuration.workerCode)
	logger = make(l4g.Logger)
	fileLogWriter := l4g.NewFileLogWriter(logFileName, false)
	fileLogWriter.SetFormat("[%D %T] [%L] (%S) %M")
	fileLogWriter.SetRotate(true)
	fileLogWriter.SetRotateDaily(true)

	if loglevel, found := loglevelMap[configuration.loglevel]; found {
		logger.AddFilter("file", loglevel, fileLogWriter)
		if loglevel == l4g.DEBUG {
			logger.AddFilter("file", loglevel, fileLogWriter)
		}
	} else {
		// default is INFO
		logger.AddFilter("file", l4g.INFO, fileLogWriter)
	}
	logger.Info("==========  start worker [%s] =========", VERSION)
	logger.Info(configuration.workerId)

	worker_net.SetLogger(logger)
	vitata_status.SetLogger(logger)

	workerNet = worker_net.NewWorkerNet()
	workerNet.SetSocketHWM(configuration.sndhwmValue, configuration.rcvhwmValue)
	workerNet.SetIntervalTime(configuration.intervalTime)
	workerNet.SetNet(configuration.proxyRouterAddr, configuration.workerId)
	workerNet.SetProcessFuncMap()
	worker_net.SetCallBackFunc(onPingMsg, onUnRegisterAppMsg, onConnectMsg,
		onBroadcastEventMsg, onRequestInitMsg, onRequestEventMsg)

	vitataStatus = vitata_status.NewVitataStatus(
		configuration.workerId,
		configuration.redisServerAddress,
		configuration.redisServerPort,
	)
	vitataStatus.SetAppsUpdateFunc()
	bRet := vitata_status.InitRedis(configuration.redisServerAddress, configuration.redisServerPort)
	if bRet == false {
		fmt.Printf("Can not connect to redis: %s:%s\n", configuration.redisServerAddress, configuration.redisServerPort)
		os.Exit(1)
	}
	vitata_status.SetCallBackFunc(sendRequestEventToClient, sendBroadcastRequestToClient)

	aliveInstances = make(map[string]bool)
	curTimestamp = time.Now()
	lastTimestamp = time.Now()
	aliveInstances = vitataStatus.GetInstancesByWorkerId(workerId)
	logger.Info("aliveInstance init: %s.", serialize(aliveInstances))
}

func main() {
	for {
		time.Sleep(3*time.Second)
		workerNet.BackgroundTask()
		vitataStatus.BackgroundTask()
		curTimestamp = time.Now()
		//每个10s检查一次instance alive
		if curTimestamp.Sub(lastTimestamp).Nanoseconds() > CHECK_INSTANCE_ALIVE_INTERVAL {
			checkInstanceAlive()
			lastTimestamp = curTimestamp
		}
	}
	//workerNet.PreDisable()
}
