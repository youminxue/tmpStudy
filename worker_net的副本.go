//  Vitata
//
//  @file     worker_net.go
//  @author   cf <feng.chen@duobei.com>
//  @date     Wed Apr 23 10:12:00 2014
//  @version  golang 1.1.2 zeromq 3.2.4
//  @brief    Rewrite the worker_net.lua in luajit 2.0.2 and lua 5.1
//
//  http://www.duobei.com
//  http://www.wangxiaotong.com
//  http://www.jiangzuotong.com
//

package worker_net

import (
	"encoding/json"
	"fmt"
	zmq "github.com/alecthomas/gozmq"
	simplejson "github.com/bitly/go-simplejson"
	l4g "github.com/cfxks1989/log4go"
	"strconv"
	"strings"
	"sync"
	"time"
	common "worker/common"
)

var lastKeepAliveTimestamp time.Time
var curClock time.Time
var preClock time.Time

type WorkerNet struct {
	sndhwmValue  int
	rcvhwmValue  int
	intervalTime int
	context      *zmq.Context
	dealer       *zmq.Socket
	pollItems    zmq.PollItems
	routeAddr    string
}

var (
	_mutex     sync.Mutex
	_workerNet *WorkerNet
	_logger    l4g.Logger
)

var broadcastCount int
var requestCount int
var totalCount int
var channelMsgCount int

const (
	KEEP_ALIVE_INTERNAL = 5e9
	intervalTime        = 6e10
)

func serialize(input interface{}) (result string) {
	resultByte, _ := json.Marshal(input)
	result = string(resultByte)
	return
}

func NewWorkerNet() *WorkerNet {
	_mutex.Lock()
	defer _mutex.Unlock()
	if _workerNet == nil {
		_logger.Debug("WorkerNet is nil, need to init.")
		_workerNet = new(WorkerNet)
	}
	return _workerNet
}

func SetLogger(logger l4g.Logger) {
	_mutex.Lock()
	defer _mutex.Unlock()
	_logger = logger
}

func printWarn(warnMsg string) {
	if _logger != nil {
		_logger.Warn(warnMsg)
	} else {
		fmt.Println(warnMsg)
	}
}

func (workerNet *WorkerNet) SetSocketHWM(sndhwmValue, rcvhwmValue int) {
	workerNet.sndhwmValue = sndhwmValue
	workerNet.rcvhwmValue = rcvhwmValue
}

func (workerNet *WorkerNet) SetIntervalTime(intervalTime int) {
	workerNet.intervalTime = intervalTime
}

func (workerNet *WorkerNet) SetNet(routeAddr string, workerId string) {
	_logger.Info("WorkerNet SetNet -> routerAddr[%s] workerId[%s]", routeAddr, workerId)
	workerNet.init(routeAddr, workerId)
}

func (w WorkerNet) sendMessage(sockIdentity, cliIdentity, msgPrefix string, msgBodyTable common.CustomMap) {
	if len(msgBodyTable) == 0 {
		return
	}
	msgBodyByte, err := json.Marshal(msgBodyTable)
	if err == nil {
		sendMsg := msgPrefix + string(msgBodyByte)
		_logger.Debug("sendMessage: sockIdentity: [ %v ] , cliIdentity: [ %s ], sendMsg: [ %s ].",
			sockIdentity, cliIdentity, sendMsg)
		sndMessage := make([][]byte, 0)
		sndMessage = append(sndMessage, []byte(sockIdentity))
		sndMessage = append(sndMessage, []byte(cliIdentity))
		sndMessage = append(sndMessage, []byte(sendMsg))
		sndErr := w.dealer.SendMultipart(sndMessage, 0)
		if sndErr != nil {
			_logger.Error("Send Message error: %s. ", serialize(msgBodyTable))
		}
		lastKeepAliveTimestamp = time.Now()
	} else {
		_logger.Error("json decode error when sendMessage: %s.", serialize(msgBodyTable))
	}
}

func (w WorkerNet) SendReplyInitMessage(sockIdentity, cliIdentity string, msgBodyTable common.CustomMap) {
	//_logger.Debug("sendReplyInitMessage: sockIdentity: [ %s ] , cliIdentity: [ %s ], msgBodyTable: [ %s ].",
	//sockIdentity, cliIdentity, serialize(msgBodyTable))
	w.sendMessage(sockIdentity, cliIdentity, REP_MSG_TYPE_INIT_PREFIX, msgBodyTable)
}

func (w WorkerNet) SendReplyEventMessage(sockIdentity, cliIdentity string, msgBodyTable common.CustomMap) {
	//_logger.Debug("sendReplyEventMessage: sockIdentity: [ %s ] , cliIdentity: [ %s ], msgBodyTable: [ %s ].",
	//sockIdentity, cliIdentity, serialize(msgBodyTable))
	w.sendMessage(sockIdentity, cliIdentity, REP_MSG_TYPE_EVENT_PREFIX, msgBodyTable)
}

func (w WorkerNet) SendBroadcastMessage(sockIdentity, cliIdentity string, msgBodyTable common.CustomMap) {
	//_logger.Debug("SendBroadcastMessage: sockIdentity: [ %s ] , cliIdentity: [ %s ], msgBodyTable: [ %s ].",
	//	sockIdentity, cliIdentity, serialize(msgBodyTable))
	w.sendMessage(sockIdentity, cliIdentity, SUB_MSG_TYPE_UPDATE_PREFIX, msgBodyTable)
}

// processCONNMsg
// just only process client connect state
/*
 //proxy_apps xpub get msg when first sub client with a specified channel connect or last sub client disconnect
 //{ proxy_apps will give the msg to worker, tell the redis key about the channel should be clear first before
 //other operation on the channel key
*/
func processConnectMsg(sockIdentity, cliIdentity, msgCmd, msgData string) {
	// msgData is appName
	_logger.Debug("processConnectMsg: %s | %s | %s| %s .", sockIdentity, cliIdentity, msgCmd, msgData)
	connState := CONN_STATE_NOTIFY_PREFIX + ":" + msgCmd + ":"
	if connState == CONN_STATE_DISCONN_PREFIX {
		connectCallBackFunc(msgData, false) // appName
	} else if connState == CONN_STATE_CONNECT_PREFIX {
		connectCallBackFunc(msgData, true) // appName
	}
}

/*
 * processRequestMsg
 * just only process REQ:$cmd:$appName
 * msgData is appName
 */
func (w WorkerNet) processRequestMsg(sockIdentity, cliIdentity, msgCmd, msgData string) {
	requestCount += 1
	//_logger.Debug("processRequestMsg: %s | %s | %s | %s .", sockIdentity, cliIdentity, msgCmd, msgData)
	msgHeadInfo := REQ_MSG_TYPE_PREFIX + ":" + msgCmd + ":"

	//REQ_MSG_TYPE_INIT_PREFIX = REQ:INIT:
	if msgHeadInfo == REQ_MSG_TYPE_INIT_PREFIX {
		//初始化VM消息
		retMsgTable := requestInitCallBackFunc(sockIdentity, cliIdentity, msgData)
		if 0 == len(retMsgTable) {
			_logger.Warn("requestInitCallBackFunc failed: %s .", msgData)
			return
		}
		w.SendReplyInitMessage(sockIdentity, cliIdentity, retMsgTable)
		return

		//REQ_MSG_TYPE_EVENT_PREFIX = REQ:EVENT:
	} else if msgHeadInfo == REQ_MSG_TYPE_EVENT_PREFIX {
		if msgData == "" {
			return
		}

		msgDataJson, _ := simplejson.NewJson([]byte(msgData))
		if msgDataJson == nil {
			_logger.Error("msgData [%s] is ERROR.", msgData)
			return
		}

		//发送的点对点消息args一定有三个参数
		if t := msgDataJson.Get("event").Get("args").GetIndex(2).Get("t").MustString(); t == "RB" {
			channelMsgCount += 1
		}

		eventMsg, err := msgDataJson.Map()
		if err != nil || len(eventMsg) == 0 {
			_logger.Error("req event json decode error: msgData: %s , info: %s .", msgData, serialize(eventMsg))
			return
		}
		retMsgTable := requestEventCallBackFunc(sockIdentity, cliIdentity, eventMsg)
		if 0 == len(retMsgTable) {
			_logger.Warn("requestEventCallBackFunc failed: %s.", serialize(eventMsg))
			return
		}
		w.SendReplyEventMessage(sockIdentity, cliIdentity, retMsgTable)
		return
		//REQ_MSG_TYPE_PING_PREFIX = REQ:PING：
	} else if msgHeadInfo == REQ_MSG_TYPE_PING_PREFIX {
		/*
			处理ping消息
			msgHeadInfo = REQ:PING:
			cliIdentity: vmapp:$instanceIP:$instancePID
			msgData: $instanceIP:$instancePID
		*/
		//_logger.Debug("Ping Message from: %s", cliIdentity)
		if strings.HasPrefix(cliIdentity, "vmapp") {
			pingCallBackFunc(msgData)
		} else {
			_logger.Warn("pingCallBackFunc could not call: cliIdentity errorcliIdentity[ %s ] msgData[ %s ]",
				cliIdentity, serialize(msgData))
		}
		//REQ_MSG_TYPE_UNREGAPP_PREFIX = REQ:UNREGAPP：
	} else if msgHeadInfo == REQ_MSG_TYPE_UNREGAPP_PREFIX {
		/*
			处理UNREGAPP消息
			msgHeadInfo = REQ:UNREGAPP:
			cliIdentity: vmapp:$instanceIP:$instancePID
			msgData: $instanceIP:$instancePID:$appName
		*/
		if strings.HasPrefix(cliIdentity, "vmapp") {
			msgList := strings.SplitN(msgData, ":", -1)
			if len(msgList) == 3 {
				_logger.Info("UNREGAPP: msgData[%s]", msgData)
				unRegisterAppCallBackFunc(msgList[0], msgList[1], msgList[2])
			} else {
				_logger.Warn("unRegisterAppCallBackFunc could not call: msgList error.  cliIdentity[ %s ]"+" msgData[ %s ]",
					cliIdentity, serialize(msgData))
			}
		} else {
			_logger.Warn("unRegisterAppCallBackFunc could not call: cliIdentity error.  cliIdentity[ %s ]"+" msgData[ %s ]",
				cliIdentity, serialize(msgData))
		}
	} else {
		_logger.Warn("processREQMsg msgHeadInfo dismatch: %s , data: %s.", msgHeadInfo, msgData)
	}
}

// processBroadcastMsg
// just only process PUB:UPDATE:xxx

func (w WorkerNet) processBroadcastMsg(sockIdentity, cliIdentity, msgCmd, msgData string) {
	broadcastCount += 1
	_logger.Debug("processBroadcastMsg: %s | %s | %s | %s .", sockIdentity, cliIdentity, msgCmd, msgData)
	if (PUB_MSG_TYPE_PREFIX + ":" + msgCmd + ":") == PUB_MSG_TYPE_UPDATE_PREFIX {
		// msgData is eventMsg
		if msgData == "" {
			return
		}
		msgDataJson, _ := simplejson.NewJson([]byte(msgData))
		if msgDataJson == nil {
			_logger.Error("msgData[%s] is ERROR.", msgData)
			return
		}

		eventMsg, err := msgDataJson.Map()

		if _, found := eventMsg["event"]; !found || err != nil || len(eventMsg) == 0 {
			_logger.Error("processBroadcastMsg msgData json decode error,  msgData: %s.", msgData)
			return
		}
		retMsgTable := broadcastCallBackFunc(eventMsg)
		if retMsgTable == nil {
			_logger.Info("No need to Send to VM: %s", serialize(eventMsg))
			return
		} else if 0 == len(retMsgTable) {
			_logger.Warn("broadcastCallBackFunc failed: %s. ", serialize(eventMsg))
			return
		}
		w.SendBroadcastMessage(sockIdentity, cliIdentity, retMsgTable)
	}
}

var processFuncMap map[string]func(string, string, string, string)

func (w WorkerNet) SetProcessFuncMap() {
	processFuncMap = make(map[string]func(string, string, string, string))
	processFuncMap[CONN_STATE_NOTIFY_PREFIX] = processConnectMsg
	processFuncMap[PUB_MSG_TYPE_PREFIX] = w.processBroadcastMsg
	processFuncMap[REQ_MSG_TYPE_PREFIX] = w.processRequestMsg
}

func (w *WorkerNet) findFunctionAndExcute(sockIdentity, cliIdentity, msg string) {
	//msg is 'msgType:msgCmd:msgData'
	//_logger.Debug("findFunctionAndExcute")
	msgParts := strings.SplitN(msg, ":", 3)
	if len(msgParts) != 3 {
		_logger.Warn("message is incomplete!")
	}
	msgType := msgParts[0]
	msgCmd := msgParts[1]
	msgData := msgParts[2]

	if msgData == "" {
		_logger.Warn("try to find msg data but error: %s. ", msg)
		return
	}

	if processFunc, found := processFuncMap[msgType]; found {
		processFunc(sockIdentity, cliIdentity, msgCmd, msgData)
	} else {
		_logger.Error("Can not find processFunc in processFuncMap[%s]", msgType)
	}
	return
}

func (w *WorkerNet) dealerZMQPollIn() {
	totalCount += 1
	parts, recvErr := w.dealer.RecvMultipart(0)
	if recvErr != nil {
		_logger.Error("dealerZMQPollIn: frontRouter RecvMultipart error: %s.", recvErr.Error())
	}
	if len(parts) < 2 {
		_logger.Warn("dealer jsut recv one frame for sockIdentity: %s.", string(parts[0]))
	}
	if len(parts) < 3 {
		_logger.Warn("dealer jsut recv one frame for cliIdentity: %s.", string(parts[1]))
	}
	sockIdentity := string(parts[0])
	cliIdentity := string(parts[1])
	msg := string(parts[2])
	//_logger.Debug("sockIdentity: %s | cliIdentity: %s | msg: %s .", sockIdentity, cliIdentity, msg)
	w.findFunctionAndExcute(sockIdentity, cliIdentity, msg)
}

var (
	pingCallBackFunc          func(string)
	unRegisterAppCallBackFunc func(string, string, string)
	connectCallBackFunc       func(string, bool)
	broadcastCallBackFunc     func(common.CustomMap) common.CustomMap
	requestInitCallBackFunc   func(string, string, string) common.CustomMap
	requestEventCallBackFunc  func(string, string, common.CustomMap) common.CustomMap
)

func SetCallBackFunc(initPingCallBackFunc func(string), initUnRegisterAppCallBackFunc func(string, string, string),
	initConnectCallBackFunc func(string, bool), initBroadcastCallBackFunc func(common.CustomMap) common.CustomMap,
	initRequestInitCallBack func(string, string, string) common.CustomMap,
	initRequestEventCallBack func(string, string, common.CustomMap) common.CustomMap) {
	pingCallBackFunc = initPingCallBackFunc
	unRegisterAppCallBackFunc = initUnRegisterAppCallBackFunc
	connectCallBackFunc = initConnectCallBackFunc
	broadcastCallBackFunc = initBroadcastCallBackFunc
	requestInitCallBackFunc = initRequestInitCallBack
	requestEventCallBackFunc = initRequestEventCallBack
}

func (w *WorkerNet) init(routerAddr string, dealerIdentity string) {
	_logger.Debug("init socket.")
	if w.context != nil {
		if w.pollItems != nil {
			if w.dealer != nil {
				w.pollItems = nil
			}
		}
		if w.dealer != nil {
			w.dealer.Close()
		}
		w.context.Close()
	}
	w.context, _ = zmq.NewContext()
	w.context.SetIOThreads(1)
	var err error
	w.dealer, err = w.context.NewSocket(zmq.DEALER)
	if err != nil {
		_logger.Error("w.dealer creat error: %v", err)
	}
	w.dealer.SetSndHWM(w.sndhwmValue)
	w.dealer.SetHWM(w.rcvhwmValue)
	w.dealer.SetLinger(0)
	w.dealer.SetSockOptString(zmq.IDENTITY, dealerIdentity)
	_logger.Info("worker_net ini routerAddr: " + routerAddr)
	err = w.dealer.Connect(routerAddr)
	if err != nil {
		_logger.Error("w.dealer connect error: %v", err)
	}
	err = w.dealer.Send([]byte(STATUS_WORKER_KEEPALIVE), 0) // now send first alive msg
	if err != nil {
		_logger.Error("w.dealer send error: %v", err)
	}
	w.pollItems = zmq.PollItems{
		zmq.PollItem{Socket: w.dealer, Events: zmq.POLLIN},
	}

	lastKeepAliveTimestamp = time.Now()
	preClock = time.Now() //common_api.getClocks()
	curClock = preClock
}

func (w *WorkerNet) PreDisable() {
	w.dealer.Close()
	w.context.Close()
	w.pollItems = nil
	w.pollItems = nil
	w.context = nil
}

func (w *WorkerNet) BackgroundTask() bool {

	count, err := zmq.Poll(w.pollItems, 5e8)

	if err != nil {
		_logger.Error("poller error: %v", err)
		return false
	}
	for index := 0; index < count; index++ {
		switch {
		case w.pollItems[0].REvents&zmq.POLLIN != 0:
			w.dealerZMQPollIn()
			break
		}
	}

	curClock = time.Now()
	//每个intervalTime时间输出所有收发消息（默认间隔是60s（1min））
	if curClock.Sub(preClock).Nanoseconds() >= intervalTime {
		preClock = curClock
		_logger.Info(" ============= TOTAL      COUNT " + strconv.Itoa(totalCount))
		_logger.Info(" ============= REQUEST    COUNT " + strconv.Itoa(requestCount))
		_logger.Info(" ============= BROADCAST  COUNT " + strconv.Itoa(broadcastCount))
		_logger.Info(" ============= CHANNELMSG COUNT " + strconv.Itoa(channelMsgCount))
		totalCount = 0
		broadcastCount = 0
		requestCount = 0
		channelMsgCount = 0
	}
	//在系统空闲时段： 每个5秒发送一次keepalive消息给proxy-apps
	if curClock.Sub(lastKeepAliveTimestamp).Nanoseconds() >= KEEP_ALIVE_INTERNAL {
		if w.dealer != nil {
			w.dealer.Send([]byte(STATUS_WORKER_KEEPALIVE), 0)
			lastKeepAliveTimestamp = curClock
		}
	}
	return true
}
