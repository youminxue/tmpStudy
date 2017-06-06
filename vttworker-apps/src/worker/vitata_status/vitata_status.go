//  Vitata
//
//  @file     vitata_status.go
//  @author   cf <feng.chen@duobei.com>
//  @date     Wed Apr 23 10:12:00 2014
//  @version  golang 1.2.1
//  @brief    Rewrite the vitata_status.lua in luajit 2.0.2 and lua 5.1
//
//  http://www.duobei.com
//  http://www.wangxiaotong.com
//  http://www.jiangzuotong.com
//

package vitata_status

import (
	"encoding/json"
	"fmt"
	simplejson "github.com/bitly/go-simplejson"
	l4g "github.com/cfxks1989/log4go"
	Redis "github.com/garyburd/redigo/redis"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	common "worker/common"
)

type AliveInstanceMap map[string]bool

// redis
type AliveInfo struct {
	WorkerId  string    `json:"workerId"`
	Timestamp time.Time `json:"timestamp"`
}

type VitataStatus struct {
	redisAddress     string
	aliveInstanceMap AliveInstanceMap
	curTimestamp     time.Time
	lastTimestamp    time.Time
	workerId         string
}

type UserClientId struct {
	protocolId string
	PID        string
	IP         string
}

func (userClientId UserClientId) struct2Map() map[string]interface{} {
	out := make(map[string]interface{})
	vType := reflect.TypeOf(userClientId)
	vValue := reflect.ValueOf(userClientId)
	for i := 0; i < vType.NumField(); i++ {
		field := vType.Field(i)
		if field.Name == "protocolId" {
			out[field.Name], _ = strconv.Atoi(vValue.FieldByName(field.Name).String())
		} else {
			out[field.Name] = vValue.FieldByName(field.Name).String()
		}
	}
	return out
}
func (u *UserClientId) Map2Struct(inputMap map[string]string) bool {
	protocolId, found1 := inputMap["protocolId"]
	PID, found2 := inputMap["PID"]
	IP, found3 := inputMap["IP"]
	if found1 && found2 && found3 {
		u.protocolId = protocolId
		u.PID = PID
		u.IP = IP
		return true
	}
	return false
}
func (u *UserClientId) MapToStruct(inputMap map[string]interface{}) bool {
	protocolId, found1 := inputMap["protocolId"]
	PID, found2 := inputMap["PID"]
	IP, found3 := inputMap["IP"]

	if found1 && found2 && found3 {
		u.protocolId = protocolId.(json.Number).String()
		u.PID = PID.(string)
		u.IP = IP.(string)
		return true
	}
	return false
}

func (u UserClientId) isEmpty() bool {
	if u.PID == "" || u.IP == "" {
		return true
	} else {
		return false
	}
}

var (
	_vitataStatus *VitataStatus
	_mutex        sync.Mutex
	_logger       l4g.Logger
)

const (
	CHAT_MESSAGE_HISTORY_SIZE = 30
	CHAT_MESSAGE_MAX_SIZE     = 200
)

//type in the redis
const (
	R_SET_TYPE    = "set"
	R_ZSET_TYPE   = "zset"
	R_HASH_TYPE   = "hash"
	R_LIST_TYPE   = "list"
	R_NONE_TYPE   = "none"
	R_STRING_TYPE = "string"
)

//the command of redis
const (
	KEYS      = "KEYS"
	SET       = "SET"
	GET       = "GET"
	DEL       = "DEL"
	EXPIRE    = "EXPIRE"
	MULTI     = "MULTI"
	SMEMBERS  = "SMEMBERS"
	SISMEMBER = "SISMEMBER"
	SREM      = "SREM"
	HGETALL   = "HGETALL"
	HSET      = "HSET"
	HDEL      = "HDEL"
	SCARD     = "SCARD"
	SADD      = "SADD"
	TYPE      = "TYPE"
	EXEC      = "EXEC"
	RPUSH     = "RPUSH"
	LTRIM     = "LTRIM"
	LRANGE    = "LRANGE"
)

//fixed: 43200e9 => 43200
const EXPIRE_TIME = 43200 //12 hour

var redis Redis.Conn = nil
var redisAddress string

func getRedis() Redis.Conn {
	if redis == nil {
		if redisAddress == "" {
			return redis
		}
		redis, _ = Redis.Dial("tcp", redisAddress)
	} else {
		result, err := redis.Do("ping")
		if result != "PONG" || err != nil {
			_logger.Error("Redis ping failure. To be reconnected")
			redis = nil
		}
	}
	return redis
}

/*
params		:input []string
return		:result map[string]string.
description	:convert string list into Map.
*/
func list2Map(input []string) map[string]string {
	if len(input)%2 != 0 {
		_logger.Debug("The length is not even.")
		return nil
	}
	result := make(map[string]string)
	for i := 0; i < len(input); {
		result[input[i]] = input[i+1]
		i = i + 2
	}
	return result
}

/*
params		:input []interface{}
return		:result map[string]interface{}.
description	:convert string list into Map.
*/
func interface2Map(input []interface{}) map[string]interface{} {
	if len(input)%2 != 0 {
		_logger.Debug("The length is not even.")
		return nil
	}
	result := make(map[string]interface{})
	for i := 0; i < len(input); {
		result[string(input[i].([]byte))] = input[i+1]
		i = i + 2
	}
	return result
}

/*
params		:inputStruct interface{}.The member of the struct is string
return		:result map[string]interface{}.
description	:convert struct  into map[string]string.
*/
func struct2Map(inputStruct interface{}) map[string]string {
	outputMap := make(map[string]string)
	vType := reflect.TypeOf(inputStruct)
	vValue := reflect.ValueOf(inputStruct)
	for i := 0; i < vType.NumField(); i++ {
		field := vType.Field(i)
		outputMap[field.Name] = vValue.FieldByName(field.Name).String()
	}
	return outputMap
}

/*
params		:inputStruct interface{}.The member of the struct is bool
return		:result map[string]interface{}.
description	:convert struct  into map[string]interface{}.
*/
func boolstructToMap(inputStruct interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	vType := reflect.TypeOf(inputStruct)
	vValue := reflect.ValueOf(inputStruct)
	for i := 0; i < vType.NumField(); i++ {
		field := vType.Field(i)
		out[field.Name] = vValue.FieldByName(field.Name).Bool()
	}
	return out
}

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

/*
params		:reply interface{}, err error.
return		:[]string, error.
description	: convert the reply of redis into stringList
			 This function is used to fix the bug in the redigo package.
*/
func interface2Strings(reply interface{}, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}

	switch reply := reply.(type) {
	case []interface{}:
		result := make([]string, len(reply))
		for i := range reply {
			if reply[i] == nil {
				continue
			}
			switch reply[i].(type) {
			case []byte:
				result[i] = string(reply[i].([]byte))
			case string:
				result[i] = reply[i].(string)
			default:
				return nil, fmt.Errorf("redigo: unexpected element type for Strings, got type %T", reply[i])
			}
		}
		return result, nil
	case nil:
		return nil, fmt.Errorf("%T", err)
	default:
		return nil, fmt.Errorf("%T", reply)
	}
	return nil, fmt.Errorf("redigo: unexpected type for Strings, got type %T", reply)
}

/*
params		:reply interface{}, err error.
return		:string, error.
description	: convert the reply of redis into string
*/
func interface2String(input interface{}) (result string, err error) {
	if input == nil {
		err = fmt.Errorf("interface2String: the input is nil.")
		return
	}
	err = nil
	switch input.(type) {
	case []byte:
		result = string(input.([]byte))
	case string:
		result = input.(string)
	case int:
		result = strconv.Itoa(input.(int))
	case int64:
		result = strconv.FormatInt(input.(int64), 10)
	case uint64:
		result = strconv.FormatUint(input.(uint64), 10)
	default:
		result = ""
		err = fmt.Errorf("interface2String: unexpected element type for String, got type %T", input)
	}
	return
}

//When all user in the room is quit, we need to clear the room.
//So this function is going to delete the KEYS in redis.

const DELAY_DELETE_TIME = 30

func (v VitataStatus) ClearRoom(appName string, forceKill bool) {
	versionKey := VTT_VERSION_PREFIX + appName                             //版本号
	userSetKey := VTT_USERSET_PREFIX + appName                             //用户集合
	stuSetKey := VTT_STUSET_PREFIX + appName                               //学生uid集合
	micUserSetKey := VTT_MICUSERSET_PREFIX + appName                       //麦克风集合
	cameraUserSetKey := VTT_CAMERAUSERSET_PREFIX + appName                 //摄像头集合
	chatMsgKey := VTT_CHATMSG_PREFIX + appName                             //聊天记录列表
	denyChatUserSetKey := VTT_DENYCHATUSERSET_PREFIX + appName             //禁言用户集合
	denyMicUserSetKey := VTT_DENYMICUSERSET_PREFIX + appName               //语音禁言用户集合
	presentKey := VTT_PRESENT_PREFIX + appName                             //PPT hash
	presentationLinesKey := VTT_PRESENTATIONLINES_PREFIX + appName         //PPT划线列表
	presentationScrollPosKey := VTT_PRESENTATIONSCROLLPOS_PREFIX + appName //PPT滚动位置
	presentationTextsKey := VTT_PRESENTATIONTEXTS_PREFIX + appName         // PPT写字列表
	roomActivityPolicyKey := VTT_ROOMACTIVITYPOLICY_PREFIX + appName       //房间策略
	generalStatusKey := VTT_GENERALSTATUS_PREFIX + appName                 //通用状态
	optKey := VTT_OPT_PREFIX + appName                                     //操作列表

	//VTT:USER:$appName:*
	userPatter := VTT_USER_PREFIX + appName + ":*"
	userKeyList, _ := Redis.Strings(redis.Do(KEYS, userPatter))
	redis.Send(MULTI)
	if forceKill {
		redis.Send(DEL, versionKey, userSetKey, stuSetKey, micUserSetKey,
			cameraUserSetKey, chatMsgKey, denyChatUserSetKey, denyMicUserSetKey,
			presentKey, presentationLinesKey, presentationScrollPosKey,
			presentationTextsKey, roomActivityPolicyKey, generalStatusKey, optKey)

	} else {
		redis.Send(EXPIRE, optKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, versionKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, userSetKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, stuSetKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, micUserSetKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, cameraUserSetKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, chatMsgKey, DELAY_DELETE_TIME)
		redis.Send(EXPIRE, denyChatUserSetKey, DELAY_DELETE_TIME)
		redis.Send(DEL, denyMicUserSetKey)
		// redis.Send(EXPIRE, presentKey, DELAY_DELETE_TIME)
		// redis.Send(EXPIRE, presentationLinesKey, DELAY_DELETE_TIME)
		// redis.Send(EXPIRE, presentationScrollPosKey, DELAY_DELETE_TIME)
		// redis.Send(EXPIRE, presentationTextsKey, DELAY_DELETE_TIME)
		// redis.Send(EXPIRE, roomActivityPolicyKey, DELAY_DELETE_TIME)
		// redis.Send(EXPIRE, generalStatusKey, DELAY_DELETE_TIME)
	}
	roomId := strings.Replace(appName, "apps/", "", 1)
	redis.Send(SREM, VTT_ROOMIDLIST_KEY, roomId)
	for userKey := range userKeyList {
		redis.Send(DEL, userKey)
	}
	redis.Do(EXEC)
}

/*
	重置房间到空的状态
*/
func (v VitataStatus) ResetRoom(appName string) {
	_logger.Info("reset appName = [%s]", appName)
	v.ClearRoom(appName, true)
	versionKey := VTT_VERSION_PREFIX + appName
	redis.Send(MULTI)
	redis.Send(SET, versionKey, 0)
	redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
	redis.Do(EXEC)
}

/*
   return value: a string list, member is "appName"
*/
func (v VitataStatus) getAppsByIpAndPid(instanceIP string, instancePID string) ([]string, error) {
	instanceKey := VTT_INSTANCE_PREFIX + instanceIP + ":" + instancePID
	return Redis.Strings(redis.Do(SMEMBERS, instanceKey))
}

func (v VitataStatus) getUsersFromAppOfInstance(instanceIP, instancePID, appName string) ([]string, error) {
	instanceAppKey := VTT_INSTANCEAPP_PREFIX + instanceIP + ":" + instancePID + ":" + appName
	_logger.Debug("getUsersFromAppOfInstance: %s.", instanceAppKey)
	return Redis.Strings(redis.Do(SMEMBERS, instanceAppKey))
}

func (v VitataStatus) getDealerIdFromAppOfInstance(instanceIP, instancePID, appName string) (string, error) {
	dealerIdKey := VTT_DEALERID_PREFIX + instanceIP + ":" + instancePID + ":" + appName
	return Redis.String(redis.Do(GET, dealerIdKey))
}

func (v VitataStatus) getUserclientidByAppnameAndUid(appName string, uid string) UserClientId {
	userKey := VTT_USER_PREFIX + appName + ":" + uid
	userInfo, _ := Redis.Strings(redis.Do(HGETALL, userKey))
	var userClientId UserClientId
	if len(userInfo) == 0 {
		return userClientId
	}
	userInfoMap := list2Map(userInfo)
	userClientId.Map2Struct(userInfoMap)
	return userClientId
}

func (v VitataStatus) RemoveUsersFromAppOfInstance(instanceIP, instancePID, appName string, isRemoveInstance bool) {
	_logger.Info("instanceID[%s:%s] | appName[%s] | %t.", instanceIP, instancePID, appName, isRemoveInstance)
	usersToClear := make([]string, 0)
	userSet, _ := v.getUsersFromAppOfInstance(instanceIP, instancePID, appName)

	userSetKey := VTT_USERSET_PREFIX + appName
	micUserSetKey := VTT_MICUSERSET_PREFIX + appName
	cameraUserSetKey := VTT_CAMERAUSERSET_PREFIX + appName
	instanceAppKey := VTT_INSTANCEAPP_PREFIX + instanceIP + ":" + instancePID + ":" + appName
	stuSetKey := VTT_STUSET_PREFIX + appName
	for _, uid := range userSet {
		userKey := VTT_USER_PREFIX + appName + ":" + uid
		userClientId := v.getUserclientidByAppnameAndUid(appName, uid)
		if userClientId.PID == instancePID && userClientId.IP == instanceIP {
			usersToClear = append(usersToClear, uid)
			redis.Do(MULTI)
			redis.Send(DEL, userKey)
			redis.Send(SREM, userSetKey, uid)
			redis.Send(SREM, stuSetKey, uid)
			//micList & camList 列表中移除该uid(可能不再里面)
			//解决 appsGetDeviceList uid doesn't exists 问题
			redis.Send(SREM, micUserSetKey, uid)
			redis.Send(SREM, cameraUserSetKey, uid)
			redis.Send(SREM, instanceAppKey, uid)
			redis.Do(EXEC)
			var count int
			count, _ = Redis.Int(redis.Do(SCARD, instanceAppKey))
			if 0 == count {
				//如果相应的appName(教室)没有人，需要在instanceList中移除相应的appName并移除dealIdKey
				instanceKey := VTT_INSTANCE_PREFIX + instanceIP + ":" + instancePID
				dealerIdKey := VTT_DEALERID_PREFIX + instanceIP + ":" + instancePID + ":" + appName
				redis.Do(MULTI)
				redis.Send(SREM, instanceKey, appName)
				redis.Send(DEL, dealerIdKey)
				redis.Do(EXEC)
			}

			count, _ = Redis.Int(redis.Do(SCARD, userSetKey))
			if 0 == count {
				_logger.Info("All users in [%s] quit.", appName)
				v.ClearRoom(appName, false)
			}
		}
	}
	if len(usersToClear) == 0 {
		_logger.Info("usersToClear is empty, no user in the appName [%s]. No need to send clientsOffline", appName)
		return
	}

	roomId := strings.Replace(appName, "apps/", "", 1)
	timestamp := time.Now().Unix() * 1000
	args := make([]interface{}, 0)
	args = append(args, usersToClear)

	event := common.CustomMap{
		"type":             "BroadcastEvent",
		"roomId":           roomId,
		"functionName":     CLI_UPDATE_USERS_LEFT_FUNC,
		"args":             args,
		"timestamp":        timestamp,
		"serverAddress":    instanceIP,
		"isRemoveInstance": isRemoveInstance,
	}

	eventMsgToClient := common.CustomMap{
		"event":   event,
		"appName": appName,
	}

	sockIdentity := "EVENT:" + appName
	sendBroadcastMsgCallBackFunc(sockIdentity, appName, eventMsgToClient)
}

func (v VitataStatus) RemoveInstance(instanceIP, instancePID, workerId string) {
	_logger.Warn("//////////////////////////////////////////////////////////")
	_logger.Warn("removeInstance: [ %s ] [ %s ]", instanceIP, instancePID)

	instanceSet, _ := v.getAppsByIpAndPid(instanceIP, instancePID)
	for _, appName := range instanceSet {
		v.RemoveUsersFromAppOfInstance(instanceIP, instancePID, appName, true)
	}

	instanceKey := VTT_INSTANCE_PREFIX + instanceIP + ":" + instancePID
	aliveInfoKey := VTT_ALIVEINFO_PREFIX + instanceIP + ":" + instancePID
	workerInstanceSetKey := VTT_WORKER_INSTANCESET_PREFIX + workerId
	redis.Do(MULTI)
	redis.Send(DEL, instanceKey)
	redis.Send(DEL, aliveInfoKey)
	redis.Send(SREM, workerInstanceSetKey, instanceIP+":"+instancePID)
	redis.Do(EXEC)
}

const INSTANCE_ALIVE_TIMEOUT = 10e9

func (v VitataStatus) GetInstancesByWorkerId(workerId string) map[string]bool {
	if redis == nil {
		_logger.Warn("redis is nil.")
		return nil
	}
	
	aliveInstances := make(map[string]bool)
	workerInstanceSetKey := VTT_WORKER_INSTANCESET_PREFIX + workerId //VTT:WORKER:$workerId
	
	instanceSet, _ := Redis.Strings(redis.Do(SMEMBERS, workerInstanceSetKey))
	if len(instanceSet) != 0 {
		currentTime := time.Now()
		for _, instanceId := range instanceSet {
			aliveInfoKey := VTT_ALIVEINFO_PREFIX + instanceId
			if aliveInfo := v.GetInstanceAliveInfoByInstanceId(instanceId, workerId); aliveInfo.WorkerId != "" {
				if currentTime.Sub(aliveInfo.Timestamp).Nanoseconds() > INSTANCE_ALIVE_TIMEOUT {
					_logger.Warn("instanceId [%s]:  aliveInfo[%v] is timeout, currentTime is %v",
						instanceId, aliveInfo, currentTime)
					redis.Do(MULTI)
					redis.Send(DEL, aliveInfoKey)
					redis.Send(SREM, workerInstanceSetKey, instanceId)
					redis.Do(EXEC)
					//aliveInstances[instanceId] = false
				} else {
					_logger.Info("init instanceId [%s] is alive.", instanceId)
					aliveInstances[instanceId] = true
				}

			} else {
				// if aliveInfoKey is not exist
				redis.Do(MULTI)
				redis.Send(DEL, aliveInfoKey)
				redis.Send(SREM, workerInstanceSetKey, instanceId)
				redis.Do(EXEC)
			}
		}
	}
	return aliveInstances
}

func (v VitataStatus) SetInstanceAliveInfo(instanceID, workerId string) {
	timestamp := time.Now()
	aliveInfoTable := AliveInfo{
		WorkerId:  workerId,
		Timestamp: timestamp,
	}
	aliveInfo, err := json.Marshal(aliveInfoTable)
	if err != nil {
		_logger.Error("setInstanceAlive json encode error: %s.", string(aliveInfo))
		return
	}
	aliveInfoKey := VTT_ALIVEINFO_PREFIX + instanceID
	workerInstanceSetKey := VTT_WORKER_INSTANCESET_PREFIX + workerId
	redis.Do(MULTI)
	redis.Send(SET, aliveInfoKey, string(aliveInfo))
	redis.Send(SADD, workerInstanceSetKey, instanceID)
	redis.Send(EXPIRE, aliveInfoKey, EXPIRE_TIME)
	redis.Send(EXPIRE, workerInstanceSetKey, EXPIRE_TIME)
	redis.Do(EXEC)
}

// return table {workerId:workerId, timestamp:timestamp}
func (v VitataStatus) GetInstanceAliveInfoByIpAndPid(instanceIP, instancePID, workerId string) (aliveInfo AliveInfo) {
	if instanceIP == "" || instancePID == "" {
		_logger.Warn("GetInstanceAliveInfoByIpAndPid instanceIP[%s] or instancePID[%s] is empty")
		return
	}
	instanceID := instanceIP + ":" + instancePID
	aliveInfo = v.GetInstanceAliveInfoByInstanceId(instanceID, workerId)
	return
}

// return table {workerId:workerId, timestamp:timestamp}
func (v VitataStatus) GetInstanceAliveInfoByInstanceId(instanceID, workerId string) (aliveInfo AliveInfo) {
	if instanceID == "" {
		_logger.Warn("GetInstanceAliveInfoByInstanceId instanceID is empty")
		return
	}
	aliveInfoKey := VTT_ALIVEINFO_PREFIX + instanceID
	//aliveInfoJson, _ := Redis.String(redis.Do(GET, aliveInfoKey))
	//err := json.Unmarshal([]byte(aliveInfoJson), &aliveInfo)
	aliveInfoJson, _ := Redis.Bytes(redis.Do(GET, aliveInfoKey))
	if aliveInfoJson == nil {
		return
	}
	err := json.Unmarshal(aliveInfoJson, &aliveInfo)
	if err == nil && aliveInfo.WorkerId == workerId {
		return
	}
	if aliveInfo.WorkerId != workerId {
		_logger.Warn("GetInstanceAliveInfoByInstanceId: aliveInfo's workerId dismatch. aliveInfo's workerId [%s] vs workerId [%s]",
			aliveInfo.WorkerId, workerId)
		_logger.Warn("aliveInfo: %v", aliveInfo)
	}
	return
}

func (v VitataStatus) appsGetCurrentVersion(appName string) int {
	versionKey := VTT_VERSION_PREFIX + appName //VTT:VERSION:$appName
	if redis == nil {
		_logger.Warn("redis is nil, getRedis()")
		redis = getRedis()
	}
	currentVer, err := Redis.Int(redis.Do("GET", versionKey))
	if err == nil || err.Error() == "redigo: nil returned" {
		return currentVer
	} else {
		_logger.Debug("appsGetCurrentVersion Error: %s.", err.Error())
		return -1
	}
}

// =========================================================================
/*
 update process
 每个状态更新的回调函数里定义了状态更新的闭包 execFunc,
 最后放在 updateOperation 中执行
*/
// if success, return current version after update
func (v VitataStatus) updateOperation(appName string, execFunc func() int) int {
	// check current version
	currentVer := v.appsGetCurrentVersion(appName)
	if currentVer == -1 {
		v.ResetRoom(appName)
	}
	return execFunc()
}

/*
  用户上线时，发现已存在状态中，将原先的位置删除，并发送给原先进程，将原先连断开
*/
func (v VitataStatus) kickUserIfExist(appName string, uid string, fromClient UserClientId, userClientId UserClientId) {
	if userClientId.isEmpty() {
		return
	}
	if fromClient.protocolId != userClientId.protocolId || fromClient.PID != userClientId.PID || fromClient.IP != userClientId.IP {
		if fromClient.IP != userClientId.IP || fromClient.PID != userClientId.PID {
			instanceAppKey := VTT_INSTANCEAPP_PREFIX + userClientId.IP + ":" + userClientId.PID + ":" + appName
			redis.Do(SREM, instanceAppKey, uid)
			if count, _ := Redis.Int(redis.Do(SCARD, instanceAppKey)); 0 == count {
				_logger.Info("kickUserIfExist: users in uid[%s] IP[%s] PID[%s] appName[%s] all quit.",
					uid, userClientId.IP, userClientId.PID, appName)
				instanceKey := VTT_INSTANCE_PREFIX + userClientId.IP + ":" + userClientId.PID
				redis.Do(SREM, instanceKey, appName)
			}
		}

		_logger.Info("KickConnectionEvent: uid[ %s ] appName[%s]. new connect is IP[%s] PID[%s] protocolId[%s]; old one is IP[%s] PID[%s] protocalId[%s].",
			uid, appName,
			fromClient.IP, fromClient.PID, fromClient.protocolId,
			userClientId.IP, userClientId.PID, userClientId.protocolId)
		event := common.CustomMap{
			"type":     "KickConnectionEvent",
			"clientId": userClientId.struct2Map(),
		}

		eventMsgToClient := common.CustomMap{
			"event":   event,
			"appName": appName,
		}

		cliIdentity := appName + ":" + userClientId.IP + ":" + userClientId.PID
		dealerId, err := v.getDealerIdFromAppOfInstance(userClientId.IP, userClientId.PID, appName)
		if err == nil {
			//发送踢人事件
			sendRelpyEventMsgCallBackFunc(dealerId, cliIdentity, eventMsgToClient)
		} else {
			_logger.Warn("kickUserIfExist dealerId [%s] not found for cliIdentity: %s.", dealerId, cliIdentity)
		}
	}
}

/*
   eventMsg and broadcastEvent check has check before call appsXXX API
   用户上线
*/
func (v *VitataStatus) appsUserJoin(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsUserJoin: %s.", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if eventArgs == nil {
		_logger.Warn("appsUserJoin eventMsg has wrong data: %s.", serialize(eventMsg))
		return -3
	}
	participant, ok := eventArgs[0].(map[string]interface{})
	var fromClient UserClientId
	fromClient.MapToStruct(eventMsg["fromClient"].(map[string]interface{}))

	if 0 == len(participant) || fromClient.isEmpty() || !ok {
		_logger.Warn("appsUserJoin: participant || fromClient is nil. eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	if fromClient.isEmpty() {
		_logger.Warn("appsUserJoin fromClient error.fromClient = [%s]", serialize(fromClient))
		return -3
	}

	uid := participant["uid"].(string)
	if uid == "" {
		_logger.Warn("appsUserJoin uid is nil. eventMsg = [%s] ", serialize(eventMsg))
		return -3
	}
	// userDomain doesn't need to be checked
	execFunc := func() int {
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsUserJoin execFunc: currentVer error: %d .", currentVer)
			return -1
		}

		versionKey := VTT_VERSION_PREFIX + appName
		userSetKey := VTT_USERSET_PREFIX + appName
		stuSetKey := VTT_STUSET_PREFIX + appName
		hstUserSetKey := VTT_HST_USERSET_PREFIX + appName
		userKey := VTT_USER_PREFIX + appName + ":" + uid
		instanceKey := VTT_INSTANCE_PREFIX + fromClient.IP + ":" + fromClient.PID
		instanceAppKey := VTT_INSTANCEAPP_PREFIX + fromClient.IP + ":" + fromClient.PID + ":" + appName
		redis.Send(MULTI)
		redis.Send(TYPE, userSetKey)
		redis.Send(TYPE, userKey)
		redis.Send(TYPE, instanceKey)
		redis.Send(TYPE, instanceAppKey)
		keyTypeList, _ := interface2Strings(redis.Do(EXEC))

		if 4 == len(keyTypeList) {
			if keyTypeList[0] != R_SET_TYPE && keyTypeList[0] != R_NONE_TYPE {
				_logger.Error("appsUserJoin keyTypeList dismatch: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[1] != R_HASH_TYPE && keyTypeList[1] != R_NONE_TYPE {
				_logger.Error("appsUserJoin keyTypeList dismatch: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[2] != R_SET_TYPE && keyTypeList[2] != R_NONE_TYPE {
				_logger.Error("appsUserJoin keyTypeList dismatch: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[3] != R_SET_TYPE && keyTypeList[3] != R_NONE_TYPE {
				_logger.Error("appsUserJoin keyTypeList dismatch: %s.", serialize(keyTypeList))
				return -1
			}
		} else {
			_logger.Error("appsUserJoin the length of keyTypeList[%s] is wrong. ", serialize(keyTypeList))
			return -1
		}

		userClientId := v.getUserclientidByAppnameAndUid(appName, uid)
		if !userClientId.isEmpty() {
			v.kickUserIfExist(appName, uid, fromClient, userClientId)
		}

		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsUserJoin eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsUserJoin error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		roomId := strings.Replace(appName, "apps/", "", 1)
		redis.Do(MULTI)
		for key, value := range participant {
			redis.Send(HSET, userKey, key, value)
		}
		for key, value := range fromClient.struct2Map() {
			redis.Send(HSET, userKey, key, value)
		}
		redis.Send(SADD, userSetKey, uid)
		redis.Send(SADD, instanceAppKey, uid)
		redis.Send(SADD, instanceKey, appName)

		//TODO 考虑使用zset进行存储，按需获取不同角色的数量
		//不是admin就计入人数统计
		if role, _ := participant["role"].(json.Number).Int64(); role != 3 {
			redis.Send(SADD, stuSetKey, uid)
			redis.Send(SADD, hstUserSetKey, uid)
		}
		redis.Send(SADD, VTT_ROOMIDLIST_KEY, roomId)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))

		redis.Send(EXPIRE, userKey, EXPIRE_TIME)
		redis.Send(EXPIRE, userSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, stuSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, instanceKey, EXPIRE_TIME)
		redis.Send(EXPIRE, instanceAppKey, EXPIRE_TIME)
		redis.Send(EXPIRE, hstUserSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  用户下线
*/
func (v *VitataStatus) appsUserLeft(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsUserLeft: %s.", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	var fromClient UserClientId
	fromClient.MapToStruct(eventMsg["fromClient"].(map[string]interface{}))
	if 0 == len(eventArgs) || fromClient.isEmpty() {
		_logger.Warn("appsUserLeft eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	uid, ok := eventArgs[0].(string)
	if !ok || fromClient.isEmpty() {
		_logger.Warn("appsUserLeft eventMsg has wrong data. eventMsg = [%s]", serialize(eventMsg))
		return -3
	}

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsUserLeft UserLeft execFunc: currentVer error: %d.", currentVer)
			return -1
		}
		userSetKey := VTT_USERSET_PREFIX + appName
		stuSetKey := VTT_STUSET_PREFIX + appName
		micUserSetKey := VTT_MICUSERSET_PREFIX + appName
		cameraUserSetKey := VTT_CAMERAUSERSET_PREFIX + appName
		userKey := VTT_USER_PREFIX + appName + ":" + uid
		instanceKey := VTT_INSTANCE_PREFIX + fromClient.IP + ":" + fromClient.PID
		instanceAppKey := VTT_INSTANCEAPP_PREFIX + fromClient.IP + ":" + fromClient.PID + ":" + appName

		redis.Send(MULTI)
		redis.Send(TYPE, userSetKey)
		redis.Send(TYPE, micUserSetKey)
		redis.Send(TYPE, cameraUserSetKey)
		redis.Send(TYPE, userKey)
		redis.Send(TYPE, instanceKey)
		redis.Send(TYPE, instanceAppKey)
		keyTypeList, _ := interface2Strings(redis.Do(EXEC))

		if 6 == len(keyTypeList) {
			// userKey Type doesn't need to be checked
			if keyTypeList[0] != R_SET_TYPE && keyTypeList[0] != R_NONE_TYPE {
				_logger.Error("appsUserLeft keyTypeList type dismatch: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[1] != R_SET_TYPE && keyTypeList[1] != R_NONE_TYPE {
				_logger.Error("appsUserLeft keyTypeList[1] type dismatch zset:%s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[2] != R_SET_TYPE && keyTypeList[2] != R_NONE_TYPE {
				_logger.Error("appsUserLeft keyTypeList[2] type dismatch zset: %s", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[3] == R_NONE_TYPE {
				_logger.Warn("appsUserLeft userLeft but user: %s doesn't exits", uid)
				return -1
			}
			if keyTypeList[4] != R_SET_TYPE {
				if keyTypeList[4] == R_NONE_TYPE {
					_logger.Warn("appsUserLeft userLeft instance doesn't exits. instanceKey: %s | uid: %s", instanceKey, uid)
				} else {
					_logger.Error("appsUserLeft keyTypeList[4] dismatch zset: %s.", serialize(keyTypeList))
				}
				return -1
			}
			if keyTypeList[5] != R_SET_TYPE {
				if keyTypeList[5] == R_NONE_TYPE {
					_logger.Warn("appsUserLeft userLeft instanceAppKey info doesn't exits. instanceAppKey; %s | uid: %s", instanceAppKey, uid)
				} else {
					_logger.Error("appsUserLeft keyTypeList[5] dismatch zset: %s.", serialize(keyTypeList))
				}
				return -1
			}
		} else {
			_logger.Error("appsUserJoin the length of keyTypeList[%s] is wrong. ", serialize(keyTypeList))
			return -1
		}
		userInfo, _ := Redis.Strings(redis.Do(HGETALL, userKey))
		var userClientId UserClientId
		if len(userInfo) != 0 {
			userInfoMap := list2Map(userInfo)
			userClientId.Map2Struct(userInfoMap)
			if fromClient.protocolId != userClientId.protocolId || fromClient.PID != userClientId.PID || fromClient.IP != userClientId.IP {
				_logger.Warn("appsUserLeft userLeft userClientId dismatch:  %#v vs %#v",
					fromClient, userClientId)
				_logger.Warn("fromClient: %v || userInfoMap: %v", eventMsg["fromClient"], userInfoMap)
				return -1
			}
		}

		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil || string(eventMsgJson) == "" {
			_logger.Error("appsUserLeft eventMsg with version [ %d ] json encode error", currentVer)
			_logger.Error("appsUserLeft error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(DEL, userKey)
		redis.Send(SREM, userSetKey, uid)
		redis.Send(SREM, stuSetKey, uid)
		redis.Send(SREM, micUserSetKey, uid)
		redis.Send(SREM, cameraUserSetKey, uid)
		redis.Send(SREM, instanceAppKey, uid)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)

		var count_user, count_instanceApp int
		redis.Send(SCARD, userSetKey)
		redis.Send(SCARD, instanceAppKey)
		redis.Flush()
		count_user, _ = Redis.Int(redis.Receive())
		count_instanceApp, _ = Redis.Int(redis.Receive())

		if 0 == count_instanceApp {
			//当前用户所连接的VM中的所有人都离开教室， 移除相应的key.
			_logger.Info("appsUserLeft: users in [%s] all quit.", instanceAppKey)
			dealerIdKey := VTT_DEALERID_PREFIX + fromClient.IP + ":" + fromClient.PID + ":" + appName
			instanceKey := VTT_INSTANCE_PREFIX + fromClient.IP + ":" + fromClient.PID
			redis.Do(MULTI)
			redis.Send(SREM, instanceKey, appName)
			redis.Send(DEL, dealerIdKey)
			redis.Do(EXEC)
		}

		if 0 == count_user {
			//该教室所有人都离开教室： 清空教室
			_logger.Info("appsUserLeft: all users in  [%s] all quit.", appName)
			v.ClearRoom(appName, false)
			currentVer = -2 // no need add to redis
		}

		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  聊天
*/
func (v *VitataStatus) appsPubChatMessage(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsPubChatMessage: ")
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsPubChatMessage eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	chatMessage, ok := eventArgs[0].(map[string]interface{})
	if 0 == len(chatMessage) || !ok {
		_logger.Warn("appsPubChatMessage eventMsg has wrong data: eventMsg = [%s] ", serialize(eventMsg))
		return -3
	}
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("PubChatMessage execFunc: currentVer error:  %d.", currentVer)
			return -1
		}

		chatMsgKey := VTT_CHATMSG_PREFIX + appName
		keyType, _ := Redis.String(redis.Do(TYPE, chatMsgKey))
		// userKey Type doesn't need to be checked
		if keyType != R_LIST_TYPE {
			if keyType != R_NONE_TYPE {
				_logger.Error("appsPubChatMessage keyType dismatch: %s.", keyType)
				return -1
			}
		}
		chatMsgJson := serialize(chatMessage)
		if chatMsgJson == "" {
			_logger.Error("appsPubChatMessage chatMsg json encode error: %s.", serialize(eventMsg))
			return -1
		}
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)

		if err != nil {
			_logger.Error("appsPubChatMessage eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsPubChatMessage error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(RPUSH, chatMsgKey, chatMsgJson)
		redis.Send(LTRIM, chatMsgKey, (-CHAT_MESSAGE_MAX_SIZE), -1)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, chatMsgKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  PPT划线事件
*/
func (v *VitataStatus) appsPresentationDrawLine(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsPresentationDrawLine: %s", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsPresentationDrawLine eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	presentationLine, ok := eventArgs[0].(map[string]interface{})
	if 0 == len(presentationLine) || !ok {
		_logger.Warn("appsPresentationDrawLine eventMsg has wrong data: eventMsg = [%s] ", serialize(eventMsg))
		return -3
	}
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsPresentationDrawLine execFunc: currentVer error:  %d.", currentVer)
			return -1
		}

		presentationLinesKey := VTT_PRESENTATIONLINES_PREFIX + appName
		keyType, _ := Redis.String(redis.Do(TYPE, presentationLinesKey))

		if keyType != R_LIST_TYPE {
			if keyType != R_NONE_TYPE {
				_logger.Error("appsPresentationDrawLine keyType dismatch: %s.", keyType)
				return -1
			}
		}
		presentationLineJson := serialize(presentationLine)
		if presentationLineJson == "" {
			_logger.Error("appsPresentationDrawLine chatMsg json encode error: %s.", serialize(eventMsg))
			return -1
		}
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)

		if err != nil {
			_logger.Error("appsPresentationDrawLine eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsPresentationDrawLine error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(RPUSH, presentationLinesKey, presentationLineJson)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, presentationLinesKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  PPTSCROLL位置更新
*/
func (v VitataStatus) appsPresentationSlideSCroll(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsPresentationSlideSCroll: %s. ", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsPresentationSlideSCroll eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}

	presentationScrollPos := serialize(eventArgs[0])
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsPresentationSlideSCroll execFunc: currentVer error: %d.", currentVer)
			return -1
		}
		presentationScrollPosKey := VTT_PRESENTATIONSCROLLPOS_PREFIX + appName
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsPresentationSlideSCroll eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsPresentationSlideSCroll error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(SET, presentationScrollPosKey, presentationScrollPos)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, presentationScrollPosKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  PPT写字事件
*/
func (v *VitataStatus) appsPresentationDrawText(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsPresentationDrawText: %s", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	presentationText, ok := event["args"].([]interface{})
	if 0 == len(presentationText) || !ok {
		_logger.Warn("eventMsg has wrong data: eventMsg = [%s] ", serialize(eventMsg))
		return -3
	}
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("execFunc: currentVer error:  %d.", currentVer)
			return -1
		}

		presentationTextsKey := VTT_PRESENTATIONTEXTS_PREFIX + appName
		keyType, _ := Redis.String(redis.Do(TYPE, presentationTextsKey))

		if keyType != R_LIST_TYPE {
			if keyType != R_NONE_TYPE {
				_logger.Error("keyType dismatch: %s.", keyType)
				return -1
			}
		}
		presentationTextJson := serialize(presentationText)
		if presentationTextJson == "" {
			_logger.Error("chatMsg json encode error: %s.", serialize(eventMsg))
			return -1
		}
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)

		if err != nil {
			_logger.Error("eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(RPUSH, presentationTextsKey, presentationTextJson)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, presentationTextsKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  PPT划线清屏事件
*/
func (v *VitataStatus) appsPresentationDrawClean(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsPresentationDrawClean: %s", serialize(eventMsg))

	//TODO 判断用户是否合法
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsPresentationDrawClean execFunc: currentVer error:  %d.", currentVer)
			return -1
		}

		presentationLinesKey := VTT_PRESENTATIONLINES_PREFIX + appName
		presentationTextsKey := VTT_PRESENTATIONTEXTS_PREFIX + appName
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)

		if err != nil {
			_logger.Error("appsPresentationDrawClean eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsPresentationDrawClean error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(DEL, presentationLinesKey)
		redis.Send(DEL, presentationTextsKey)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  麦克风状态切换 (开关)
  micOnAir
  micOff
  更新 micList
*/
func (v *VitataStatus) appsMicStatusSwitch(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsMicStatusSwitch ")
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	var functionName string
	if ok {
		eventArgs, _ = event["args"].([]interface{})
		functionName, _ = event["functionName"].(string)
	}

	if 0 == len(eventArgs) {
		_logger.Warn("appsMicStatusSwitch eventMsg has no args. eventMsg = [%s]", serialize(eventMsg))
		return -3
	}
	participant, ok := eventArgs[0].(map[string]interface{})
	_, found := participant["uid"]
	if 0 == len(participant) || !ok || !found {
		// 可能需要做 role imageUrl username 判断
		_logger.Warn("appsMicStatusSwitch eventMsg participant is error: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	uid := participant["uid"].(string)

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("MicStatusSwitch execFunc: currentVer error: %d.", currentVer)
			return -1
		}
		userSetKey := VTT_USERSET_PREFIX + appName       //VTT:USERSET:
		micUserSetKey := VTT_MICUSERSET_PREFIX + appName //VTT:MICUSERSET:
		userKey := VTT_USER_PREFIX + appName + ":" + uid //VTT:USER:$appName:$uid
		redis.Send(MULTI)
		redis.Send(TYPE, userSetKey)
		redis.Send(TYPE, micUserSetKey)
		redis.Send(TYPE, userKey)
		keyTypeList, _ := interface2Strings(redis.Do(EXEC))
		if 3 == len(keyTypeList) {
			if keyTypeList[0] != R_SET_TYPE && keyTypeList[0] != R_NONE_TYPE {
				_logger.Error("appsMicStatusSwitch keyTypeList[0] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[1] != R_SET_TYPE && keyTypeList[1] != R_NONE_TYPE {
				_logger.Error("appsMicStatusSwitch keyTypeList[1] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[2] != R_HASH_TYPE && keyTypeList[2] != R_NONE_TYPE {
				_logger.Error("appsMicStatusSwitch keyTypeList[2] dismatch hash: %s.", serialize(keyTypeList))
				return -1
			}
		} else {
			_logger.Error("appsUserJoin the length of keyTypeList[%s] is wrong.", serialize(keyTypeList))
			return -1
		}
		// check uid exists
		isEXits, err := Redis.Bool(redis.Do(SISMEMBER, userSetKey, uid))
		if !isEXits && err != nil {
			_logger.Warn("appsMicStatusSwitch user uid = [%s] does exists!!, will del some info.", uid)
			redis.Do(MULTI)
			redis.Send(DEL, userKey)
			redis.Send(SREM, micUserSetKey, uid)
			redis.Do(EXEC)
			return -1
		}
		currentVer = currentVer + 1
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsMicStatusSwitch eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsMicStatusSwitch error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(SET, versionKey, currentVer)
		if functionName == CLI_UPDATE_MIC_ON_AIR_FUNC {
			redis.Send(SADD, micUserSetKey, uid)
		} else if functionName == CLI_UPDATE_MIC_OFF_FUNC {
			redis.Send(SREM, micUserSetKey, uid)
		}
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, userKey, EXPIRE_TIME)
		redis.Send(EXPIRE, userSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, micUserSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  摄像头状态切换 (开关)
  cameraOnAir
  cameraOff
  更新 cameraList
*/
func (v *VitataStatus) appsCameraStatusSwitch(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsCameraStatusSwitch: %s.", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	var functionName string
	if ok {
		eventArgs, _ = event["args"].([]interface{})
		functionName, _ = event["functionName"].(string)
	}
	if 0 == len(eventArgs) || "" == functionName {
		_logger.Warn("appsCameraStatusSwitch eventMsg has no args. eventMsg=[%s].", serialize(eventMsg))
		return -3
	}
	participant := eventArgs[0].(map[string]interface{})
	_, found := participant["uid"]
	if 0 == len(participant) || !found {
		// 可能需要做 role imageUrl username 判
		_logger.Warn("appsCameraStatusSwitch eventMsg participant is error: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	uid := participant["uid"].(string)

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("CameraStatusSwitch execFunc: currentVer error: %d.", currentVer)
			return -1
		}

		userSetKey := VTT_USERSET_PREFIX + appName
		cameraUserSetKey := VTT_CAMERAUSERSET_PREFIX + appName
		userKey := VTT_USER_PREFIX + appName + ":" + uid
		redis.Send(MULTI)
		redis.Send(TYPE, userSetKey)
		redis.Send(TYPE, cameraUserSetKey)
		redis.Send(TYPE, userKey)
		keyTypeList, _ := interface2Strings(redis.Do(EXEC))

		if 3 == len(keyTypeList) {
			if keyTypeList[0] != R_SET_TYPE && keyTypeList[0] != R_NONE_TYPE {
				_logger.Error("appsCameraStatusSwitch keyTypeList[0] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[1] != R_SET_TYPE && keyTypeList[1] != R_NONE_TYPE {
				_logger.Error("appsCameraStatusSwitch keyTypeList[1] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[2] != R_HASH_TYPE && keyTypeList[2] != R_NONE_TYPE {
				_logger.Error("appsCameraStatusSwitch keyTypeList[2] dismatch hash: %s.", serialize(keyTypeList))
				return -1
			}
		} else {
			_logger.Error("appsCameraStatusSwitch the length of keyTypeList[%s] is wrong.", serialize(keyTypeList))
			return -1
		}
		// check uid exists
		isExits, _ := Redis.Bool(redis.Do(SISMEMBER, userSetKey, uid))
		if !isExits {
			_logger.Warn("appsCameraStatusSwitch user %s does not  exists!!, cameraStatus switch failed.", uid)
			redis.Do(MULTI)
			redis.Send(DEL, userKey)
			redis.Send(SREM, cameraUserSetKey, uid)
			redis.Do(EXEC)
			return -1
		}

		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsCameraStatusSwitch eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsCameraStatusSwitch error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(SET, versionKey, currentVer)
		if functionName == CLI_UPDATE_CAMERA_ON_AIR_FUNC {
			redis.Send(SADD, cameraUserSetKey, uid)
		} else {
			redis.Send(SREM, cameraUserSetKey, uid)
		}

		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, userKey, EXPIRE_TIME)
		redis.Send(EXPIRE, userSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, cameraUserSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  指定用户禁言设置
  clientAllowChatByUid
  clientDenyChatByUid
  更新 denyChatList
*/
func (v VitataStatus) appsSetChatPermission(appName string, eventMsg common.CustomMap) int {

	_logger.Debug("appsSetChatPermission")
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	var functionName string
	if ok {
		eventArgs, _ = event["args"].([]interface{})
		functionName, _ = event["functionName"].(string)
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsSetChatPermission eventMsg has no args. eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	participant := eventArgs[0].(map[string]interface{})
	_, found := participant["uid"]
	if 0 == len(participant) || !found {
		// 可能需要做 role imageUrl username 判断
		_logger.Warn("appsSetChatPermission eventMsg participant is error:  eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	uid := participant["uid"].(string)

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("ChatPermission execFunc: currentVer error: %d.", currentVer)
			return -1
		}

		userSetKey := VTT_USERSET_PREFIX + appName
		denyChatUserSetKey := VTT_DENYCHATUSERSET_PREFIX + appName
		userKey := VTT_USER_PREFIX + appName + ":" + uid
		redis.Send(MULTI)
		redis.Send(TYPE, userSetKey)
		redis.Send(TYPE, denyChatUserSetKey)
		redis.Send(TYPE, userKey)
		keyTypeList, _ := interface2Strings(redis.Do(EXEC))

		if 3 == len(keyTypeList) {
			if keyTypeList[0] != R_SET_TYPE && keyTypeList[0] != R_NONE_TYPE {
				_logger.Error("appsSetChatPermission keyTypeList[0] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[1] != R_SET_TYPE && keyTypeList[1] != R_NONE_TYPE {
				_logger.Error("appsSetChatPermission keyTypeList[1] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[2] != R_HASH_TYPE && keyTypeList[2] != R_NONE_TYPE {
				_logger.Error("appsSetChatPermission keyTypeList[2] dismatch hash: %s.", serialize(keyTypeList))
				return -1
			}
		} else {
			_logger.Error("appsSetChatPermission the length of keyTypeList[%s] is wrong.", serialize(keyTypeList))
			return -1
		}
		// check uid exists
		//isExits, _ := Redis.Bool(redis.Do(SISMEMBER, userSetKey, uid))
		//if !isExits {
		//	_logger.Warn("appsSetChatPermission user uid=[%s] does not exists!!, will del some info.", uid)
		//	redis.Do(MULTI)
		//	redis.Send(DEL, userKey)
		//	redis.Send(SREM, denyChatUserSetKey, uid)
		//	redis.Do(EXEC)
		//	return -1
		//}
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsSetChatPermission eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsSetChatPermission error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(SET, versionKey, currentVer)
		if functionName == CLI_UPDATE_DENYCHAT_UID_FUNC {
			redis.Send(SADD, denyChatUserSetKey, uid)
		} else {
			redis.Send(SREM, denyChatUserSetKey, uid)
		}
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, denyChatUserSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  指定用户语音禁言设置
  clientAllowMicByUid
  clientDenyMicByUid
  更新 denyMicList
*/
func (v VitataStatus) appsSetMicPermission(appName string, eventMsg common.CustomMap) int {

	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	var functionName string
	if ok {
		eventArgs, _ = event["args"].([]interface{})
		functionName, _ = event["functionName"].(string)
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsSetMicPermission eventMsg has no args. eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	participant := eventArgs[0].(map[string]interface{})
	_, found := participant["uid"]
	if 0 == len(participant) || !found {
		// 可能需要做 role imageUrl username 判断
		_logger.Warn("appsSetMicPermission eventMsg participant is error:  eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	uid := participant["uid"].(string)

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsSetMicPermission execFunc: currentVer error: %d.", currentVer)
			return -1
		}

		userSetKey := VTT_USERSET_PREFIX + appName
		denyMicUserSetKey := VTT_DENYMICUSERSET_PREFIX + appName
		userKey := VTT_USER_PREFIX + appName + ":" + uid
		redis.Send(MULTI)
		redis.Send(TYPE, userSetKey)
		redis.Send(TYPE, denyMicUserSetKey)
		redis.Send(TYPE, userKey)
		keyTypeList, _ := interface2Strings(redis.Do(EXEC))

		if 3 == len(keyTypeList) {
			if keyTypeList[0] != R_SET_TYPE && keyTypeList[0] != R_NONE_TYPE {
				_logger.Error("appsSetMicPermission keyTypeList[0] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[1] != R_SET_TYPE && keyTypeList[1] != R_NONE_TYPE {
				_logger.Error("appsSetMicPermission keyTypeList[1] dismatch set: %s.", serialize(keyTypeList))
				return -1
			}
			if keyTypeList[2] != R_HASH_TYPE && keyTypeList[2] != R_NONE_TYPE {
				_logger.Error("appsSetMicPermission keyTypeList[2] dismatch hash: %s.", serialize(keyTypeList))
				return -1
			}
		} else {
			_logger.Error("appsSetMicPermission the length of keyTypeList[%s] is wrong.", serialize(keyTypeList))
			return -1
		}

		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsSetMicPermission eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsSetMicPermission error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(SET, versionKey, currentVer)
		if functionName == CLI_UPDATE_DENYMIC_UID_FUNC {
			redis.Send(SADD, denyMicUserSetKey, uid)
		} else {
			redis.Send(SREM, denyMicUserSetKey, uid)
		}
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, denyMicUserSetKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  课件翻页
*/
func (v VitataStatus) appsGotoSlide(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("")
	_logger.Debug("appsGotoSlide: ")
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsGotoSlide eventMsg has wrong data: %s.", serialize(eventMsg))
		return -3
	}
	presentationId, ret1 := eventArgs[0].(json.Number).Int64()
	slidePageNumber, ret2 := eventArgs[1].(json.Number).Int64()
	if ret1 != nil || ret2 != nil {
		_logger.Warn("appsGotoSlide eventMsg has wrong data: %s.", serialize(eventMsg))
		return -3
	}

	presentKey := VTT_PRESENT_PREFIX + appName
	presentationType, _ := Redis.String(redis.Do(TYPE, presentKey))
	if presentationType != R_HASH_TYPE {
		_logger.Warn("appsGotoSlide could not go to slide: %s." + string(presentationType))
		if presentationType != R_NONE_TYPE {
			redis.Do(DEL, presentKey)
		}
		return -3
	}

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("GotoSlide execFunc: currentVer error: %d.", currentVer)
			return -1
		}

		presentation, _ := Redis.Strings(redis.Do(HGETALL, presentKey))
		presentationMap := list2Map(presentation)
		curPresentationId, _ := strconv.ParseInt(presentationMap["id"], 10, 64)
		slideCount, _ := strconv.ParseInt(presentationMap["slideCount"], 10, 64)
		_logger.Debug(curPresentationId, slideCount)
		if curPresentationId != presentationId || slidePageNumber > slideCount || slidePageNumber <= 0 {
			_logger.Warn("[WARNING] gotoSlide execFunc eventMsg may has wrong data.")
			_logger.Warn("slidePageNumber: %s.", string(slidePageNumber))
			_logger.Warn(" eventMsg = [%s].", serialize(eventMsg))
			_logger.Warn("current presentation: %s.", serialize(presentation))
			return -1
		}

		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsGotoSlide eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsGotoSlide error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		presentationLinesKey := VTT_PRESENTATIONLINES_PREFIX + appName
		presentationScrollPosKey := VTT_PRESENTATIONSCROLLPOS_PREFIX + appName
		presentationTextsKey := VTT_PRESENTATIONTEXTS_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(HSET, presentKey, "currentSlidePageNumber", slidePageNumber)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(DEL, presentationLinesKey)     //清除redis中的划线记录
		redis.Send(DEL, presentationTextsKey)     //清除redis中的写字记录
		redis.Send(DEL, presentationScrollPosKey) //清除redis中的滚动位置信息
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, presentKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  更换课件
*/
func (v VitataStatus) appsPresentationChange(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsPresentationChange: ")
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsPresentationChange eventMsg has wrong data: %s.", serialize(eventMsg))
		return -3
	}

	presentation, _ := eventArgs[0].(map[string]interface{})
	if 0 == len(presentation) {
		_logger.Warn("appsPresentationChange eventMsg has no presentation: %s.", serialize(eventMsg))
		return -3
	}

	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("PresentationChange execFunc currentVer error: %d.", currentVer)
			return -1
		}

		presentKey := VTT_PRESENT_PREFIX + appName
		_, found1 := presentation["id"]
		_, found2 := presentation["name"]
		_, found3 := presentation["uuid"]
		_, found4 := presentation["slideCount"]
		_, found5 := presentation["currentSlidePageNumber"]
		if !found1 || !found2 || !found3 || !found4 || !found5 {
			_logger.Warn("presentationChange execFunc eventMsg may has wrong data: eventMsg = [%s].", serialize(eventMsg))
			return -1
		}
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("PresentationChange eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("PresentationChange error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		presentationLinesKey := VTT_PRESENTATIONLINES_PREFIX + appName
		presentationScrollPosKey := VTT_PRESENTATIONSCROLLPOS_PREFIX + appName
		presentationTextsKey := VTT_PRESENTATIONTEXTS_PREFIX + appName
		redis.Send(MULTI)
		for key, value := range presentation {
			redis.Send(HSET, presentKey, key, value)
		}
		redis.Send(SET, versionKey, currentVer)
		redis.Send(DEL, presentationLinesKey)     //清除redis中的划线记录
		redis.Send(DEL, presentationScrollPosKey) //清除redis中的滚动位置信息
		redis.Send(DEL, presentationTextsKey)     //清除redis中的写字记录
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, presentKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  教室权限更新
*/
func (v VitataStatus) mcRoomActivityPolicyControl(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("mcRoomActivityPolicyControl: %s. ", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("mcRoomActivityPolicyControl eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	//roomActivityPolicy, _ := eventArgs[0].(map[string]interface{})
	currentPolicy := v.appsGetRoomActivityPolicy(appName)
	currentPolicyMap := make(map[string]interface{})
	json.Unmarshal([]byte(currentPolicy), &currentPolicyMap)
	for key, value := range eventArgs[0].(map[string]interface{}) {
		currentPolicyMap[key] = value
	}
	roomActivityPolicy := serialize(currentPolicyMap)
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("RoomActivityPolicyControl execFunc: currentVer error: %d.", currentVer)
			return -1
		}
		roomActivityPolicyKey := VTT_ROOMACTIVITYPOLICY_PREFIX + appName
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("RoomActivityPolicyControl eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("RoomActivityPolicyControl error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		redis.Send(MULTI)
		redis.Send(SET, roomActivityPolicyKey, roomActivityPolicy)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, roomActivityPolicyKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  教室通用状态更新
*/
func (v VitataStatus) appsSetGeneralStatus(appName string, eventMsg common.CustomMap) int {
	_logger.Debug("appsSetGeneralStatus: %s. ", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}

	newGeneralStatus, _ := eventArgs[1].(map[string]interface{})
	if 0 == len(newGeneralStatus) {
		_logger.Warn("appsSetGeneralStatus eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsSetGeneralStatus execFunc: currentVer error: %d.", currentVer)
			return -1
		}
		generalStatusKey := VTT_GENERALSTATUS_PREFIX + appName
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsSetGeneralStatus eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsSetGeneralStatus error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		set, found_1 := newGeneralStatus["SET"]
		del, found_2 := newGeneralStatus["DEL"]
		var documentId string
		if value, found_3 := newGeneralStatus["DOCID"]; found_3 {
			if docId, ok := value.(string); ok {
				documentId = string(docId)
			} else {
				documentId = "default"
			}
		} else {
			documentId = "default"
		}
		generalStatusKey = generalStatusKey + ":" + documentId

		//如果没有找到SET或者DEL键值，就不处理。
		if (found_1 || found_2) == false {
			_logger.Warn("appsSetGeneralStatus not found SET or DEL")
			return -3
		}
		redis.Send(MULTI)
		if found_1 {
			setMap, _ := set.(map[string]interface{})
			for key, value := range setMap {
				redis.Send(HSET, generalStatusKey, key, value)
			}
		}

		if found_2 {
			delList, _ := del.(map[string]interface{})
			for _, value := range delList {
				redis.Send(HDEL, generalStatusKey, value)
			}
		}
		statusListKey := VTT_GENERALSTATUSLIST_PREFIX + appName

		redis.Send(SET, versionKey, currentVer)
		redis.Send(SADD, statusListKey, documentId)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, generalStatusKey, EXPIRE_TIME)
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

/*
  教室通用状态更新
*/
func (v VitataStatus) appsCleanGeneralStatus(appName string, eventMsg common.CustomMap) int {
	_logger.Info("appsCleanGeneralStatus: %s. ", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs, _ = event["args"].([]interface{})
	}

	docId, ok := eventArgs[1].(string)
	if ok == false {
		_logger.Warn("appsCleanGeneralStatus eventMsg has wrong data: eventMsg = [%s].", serialize(eventMsg))
		return -3
	}
	if docId == "" {
		docId = "default"
	}
	execFunc := func() int {
		versionKey := VTT_VERSION_PREFIX + appName
		currentVer := v.appsGetCurrentVersion(appName)
		if currentVer < 0 {
			_logger.Error("appsCleanGeneralStatus execFunc: currentVer error: %d.", currentVer)
			return -1
		}
		generalStatusKey := VTT_GENERALSTATUS_PREFIX + appName + ":" + docId
		currentVer = currentVer + 1
		// eventMsg 增加了 version  属性
		eventMsg["version"] = currentVer
		eventMsgJson, err := json.Marshal(eventMsg)
		if err != nil {
			_logger.Error("appsCleanGeneralStatus eventMsg with version [%d] json encode error.", currentVer)
			_logger.Error("appsCleanGeneralStatus error info: %s.", string(eventMsgJson))
			return -1
		}
		optKey := VTT_OPT_PREFIX + appName
		statusListKey := VTT_GENERALSTATUSLIST_PREFIX + appName
		redis.Send(DEL, generalStatusKey)
		redis.Send(SET, versionKey, currentVer)
		redis.Send(SREM, statusListKey, docId)
		redis.Send(RPUSH, optKey, string(eventMsgJson))
		redis.Send(EXPIRE, versionKey, EXPIRE_TIME)
		redis.Send(EXPIRE, optKey, EXPIRE_TIME)
		redis.Do(EXEC)
		return currentVer
	}
	return v.updateOperation(appName, execFunc)
}

var appsUpdateFuncMap map[string]func(string, common.CustomMap) int

//设置appsUpdateFuncMap的对应值
func (v VitataStatus) SetAppsUpdateFunc() {
	appsUpdateFuncMap = make(map[string]func(string, common.CustomMap) int)
	appsUpdateFuncMap[CLI_UPDATE_USER_JOIN_FUNC] = v.appsUserJoin
	appsUpdateFuncMap[CLI_UPDATE_USER_LEFT_FUNC] = v.appsUserLeft
	appsUpdateFuncMap[CLI_UPDATE_CHAT_FUNC] = v.appsPubChatMessage
	appsUpdateFuncMap[CLI_UPDATE_MIC_ON_AIR_FUNC] = v.appsMicStatusSwitch
	appsUpdateFuncMap[CLI_UPDATE_MIC_OFF_FUNC] = v.appsMicStatusSwitch
	appsUpdateFuncMap[CLI_UPDATE_CAMERA_ON_AIR_FUNC] = v.appsCameraStatusSwitch
	appsUpdateFuncMap[CLI_UPDATE_CAMERA_OFF_FUNC] = v.appsCameraStatusSwitch
	appsUpdateFuncMap[CLI_UPDATE_ALLOWCHAT_UID_FUNC] = v.appsSetChatPermission
	appsUpdateFuncMap[CLI_UPDATE_DENYCHAT_UID_FUNC] = v.appsSetChatPermission
	appsUpdateFuncMap[CLI_UPDATE_ALLOWMIC_UID_FUNC] = v.appsSetMicPermission
	appsUpdateFuncMap[CLI_UPDATE_DENYMIC_UID_FUNC] = v.appsSetMicPermission
	appsUpdateFuncMap[CLI_UPDATE_PRESENTATION_SLIDECHANGE_FUNC] = v.appsGotoSlide
	appsUpdateFuncMap[CLI_UPDATE_PRESENTATION_CHANGE_FUNC] = v.appsPresentationChange
	appsUpdateFuncMap[CLI_UPDATE_PRESENTATION_DRAWLINE_FUNC] = v.appsPresentationDrawLine
	appsUpdateFuncMap[CLI_UPDATE_PRESENTATION_DRAWCLEAN_FUNC] = v.appsPresentationDrawClean
	appsUpdateFuncMap[CLI_UPDATE_PRESENTATION_SLIDESCROLL_FUNC] = v.appsPresentationSlideSCroll
	appsUpdateFuncMap[CLI_UPDATE_PRESENTATION_DRAWTEXT_FUNC] = v.appsPresentationDrawText
	appsUpdateFuncMap[MC_UPDATE_ROOM_ACTIVITY_POLICY_CONTROL_FUNC] = v.mcRoomActivityPolicyControl
	appsUpdateFuncMap[CLI_UPDATE_STATUS_SET_FUNC] = v.appsSetGeneralStatus
	appsUpdateFuncMap[CLI_UPDATE_STATUS_CLEAN_FUNC] = v.appsCleanGeneralStatus

}

/*
  处理状态更新事件
*/
func (v VitataStatus) AppsProcessUpdateEvent(eventMsg common.CustomMap) common.CustomMap {
	//_logger.Debug("appsProcessUpdateEvent: %s", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var functionName, appName string
	if ok {
		functionName, _ = event["functionName"].(string)
		appName, _ = eventMsg["appName"].(string)
	} else {
		_logger.Warn("eventMsg: %v  is error.", eventMsg)
	}

	if "" == functionName || "" == appName {
		_logger.Warn("vitata accessor appsProcessUpdateEvent eventMsg data error:eventMsg = [%s].", serialize(eventMsg))
		return nil
	}
	appsUpdateFunc, found := appsUpdateFuncMap[functionName]
	if !found {
		_logger.Warn("appsUpdateFunc has not been found." + serialize(eventMsg))
		return nil
	}

	// eventMsg 在 appsUpdateFunc 中会增加 version 属性, chat 事件除外
	currentVer := appsUpdateFunc(appName, eventMsg)
	if currentVer < 0 && currentVer != -2 {
		_logger.Warn("UpdateFunc for %s , %s  process error, get current version: %d. eventMsg = %s.",
			functionName, appName, currentVer, serialize(eventMsg))
		return nil
	} else if currentVer == -2 {
		_logger.Info("UpdateFunc no need: users in %s have all quit.", appName)
		return nil
	}
	return eventMsg
}

/*
  处理点对点事件
  1v1中调整视频质量 & 向admin上报信息等都是用该接口。
*/
func (v VitataStatus) AppsProcessSendToClientEvent(eventMsg common.CustomMap) bool {
	_logger.Debug("appsProcessSendToClientEvent: %s.", serialize(eventMsg))
	event, ok := eventMsg["event"].(map[string]interface{})
	var eventArgs []interface{}
	if ok {
		eventArgs = event["args"].([]interface{})
	}
	if 0 == len(eventArgs) {
		_logger.Warn("appsProcessSendToClientEvent eventMsg has no args. eventMsg = [%s].", serialize(eventMsg))
		return false
	}

	functionName := event["functionName"].(string)
	appName := eventMsg["appName"].(string)
	fromUid, _ := eventArgs[0].(string)
	toUid, _ := eventArgs[1].(string)
	sendMsg, _ := eventArgs[2].(map[string]interface{})
	if "" == functionName || "" == appName || "" == fromUid || "" == toUid || len(sendMsg) == 0 {
		_logger.Warn("appsProcessSendToClientEvent vitata_status eventMsg data error: eventMsg = [%s]. ", serialize(eventMsg))
		return false
	}

	userKey := VTT_USER_PREFIX + appName + ":" + toUid
	userInfo, _ := Redis.Strings(redis.Do(HGETALL, userKey))
	userClientId := make(map[string]interface{})
	userInfoMap := make(map[string]string)
	if len(userInfo) != 0 {
		userInfoMap = list2Map(userInfo)
		_, found1 := userInfoMap["protocolId"]
		_, found2 := userInfoMap["IP"]
		_, found3 := userInfoMap["PID"]
		if found1 && found2 && found3 {
			userClientId["protocolId"], _ = strconv.Atoi(userInfoMap["protocolId"])
			userClientId["IP"] = userInfoMap["IP"]
			userClientId["PID"] = userInfoMap["PID"]
		} else {
			_logger.Warn("appsProcessSendToClientEvent: can not get userClientId by userInfo[%s]", serialize(userInfo))
			return false
		}
	} else {
		//发送点对点消息时候，对方已经不在线。
		//_logger.Warn("appsProcessSendToClientEvent: can not get userInfo, key=[%s].", userKey)
		return false
	}
	newEvent := common.CustomMap{
		"type":         "SendToClientEvent",
		"roomId":       event["roomId"],
		"functionName": event["functionName"],
		"args":         event["args"],
		"timestamp":    event["timestamp"],
		"clientId":     userClientId,
	}
	eventMsgToClient := common.CustomMap{
		"event":   newEvent,
		"appName": appName,
	}
	cliIdentity := appName + ":" + userInfoMap["IP"] + ":" + userInfoMap["PID"]
	dealerId, _ := v.getDealerIdFromAppOfInstance(userInfoMap["IP"], userInfoMap["PID"], appName)
	if dealerId != "" {
		sendRelpyEventMsgCallBackFunc(dealerId, cliIdentity, eventMsgToClient)
	} else {
		_logger.Warn("appsProcessSendToClientEvent dealerId not found for cliIdentity: %s.", cliIdentity)
	}
	return true
}

/*
  获取 update event list, snapshot 请求来时，把需要的版本事件返回给流媒体vm
*/
func (v VitataStatus) AppsGetUpdateEventList(sockIdentity, cliIdentity, appName string,
	startVersion, endVersion int64) []common.CustomMap {
	_logger.Debug("appsGetUpdateEventList")
	if startVersion > endVersion {
		_logger.Error("appsGetUpdateEventList version range error: %d | %d.", startVersion, endVersion)
		return nil
	}
	optKey := VTT_OPT_PREFIX + appName
	optKeyType, _ := Redis.String(redis.Do(TYPE, optKey))
	if optKeyType != R_LIST_TYPE {
		if optKeyType != R_NONE_TYPE {
			_logger.Error("appsGetUpdateEventList: %s's type is %s", optKey, optKeyType)
		}
		return nil
	}

	updateEventMsgJsonList, _ := Redis.Strings(redis.Do(LRANGE, optKey, startVersion-1, endVersion-1))
	updateEventMsgList := make([]common.CustomMap, 0)

	//循环解析json，并添加到list中
	for _, updateEventMsgJson := range updateEventMsgJsonList {
		if updateEventMsgJson == "" {
			continue
		}
		result, err := simplejson.NewJson([]byte(updateEventMsgJson))
		if err != nil {
			_logger.Error("updateEventMsgJson [%s] is ERROR.", updateEventMsgJson)
			continue
		}
		updateEventMsg, err2 := result.Map()
		if err == nil && err2 == nil {
			updateEventMsgList = append(updateEventMsgList, updateEventMsg)
		}
	}
	cliIdentityInfo := strings.SplitN(cliIdentity, ":", -1)
	if 3 == len(cliIdentityInfo) {
		instanceIP := cliIdentityInfo[1]
		instancePID := cliIdentityInfo[2]
		dealerIdKey := VTT_DEALERID_PREFIX + instanceIP + ":" + instancePID + ":" + appName //VTT:DEALERID:
		redis.Do(MULTI)
		redis.Send(SET, dealerIdKey, sockIdentity)
		redis.Send(EXPIRE, dealerIdKey, EXPIRE_TIME)
		redis.Do(EXEC)
	} else {
		_logger.Error("appsGetUpdateEventList cliIdentity may be error: %s.", cliIdentity)
	}
	return updateEventMsgList
}

// =========================================================================
/*
   appsGet room status API
   获取当前 用户列表 (map) uid 为key
*/
func (v VitataStatus) appsGetAudienceList(appName string) map[string]map[string]interface{} {
	_logger.Debug("appsGetAudienceList")
	audienceList := make(map[string]map[string]interface{})
	userSetKey := VTT_USERSET_PREFIX + appName       //VTT:USERSET:$appName
	userKeyPrefix := VTT_USER_PREFIX + appName + ":" //VTT:USER:$appName
	userSet, _ := Redis.Strings(redis.Do(SMEMBERS, userSetKey))
	if 0 != len(userSet) {
		for _, uid := range userSet {
			redis.Send(HGETALL, userKeyPrefix+uid)
		}
		redis.Flush()
		for i := 0; i < len(userSet); i++ {
			reply, _ := Redis.Strings(redis.Receive())
			participantMap := list2Map(reply)
			participant := make(map[string]interface{})
			//var test Participant
			//test.Map2Struct(participantMap)
			if uid, found := participantMap["uid"]; found {
				participant["imageUrl"] = participantMap["imageUrl"]
				participant["role"], _ = strconv.Atoi(participantMap["role"])
				participant["username"] = participantMap["username"]
				participant["uid"] = uid
				audienceList[uid] = participant
			} else {
				_logger.Warn("appsGetAudienceList uid doesn't exists, reply =[%s]. ", serialize(reply))
			}
		}
	}
	_logger.Debug("%#v", audienceList)
	return audienceList
}

/*
   获取当前在线人数和历史在线人数
*/

func (v VitataStatus) appsGetUserCount(appName string) map[string]interface{} {
	result := make(map[string]interface{}, 0)
	redis.Send(SCARD, VTT_HST_USERSET_PREFIX+appName)
	redis.Send(SCARD, VTT_STUSET_PREFIX+appName)
	redis.Flush()
	hstCount, _ := Redis.Int(redis.Receive())
	onlineCount, _ := Redis.Int(redis.Receive())
	result["hst"] = hstCount
	result["online"] = onlineCount
	return result
}

/*
   获取 当前设备的用户列表
   仅被 appsGetMicList 和 appsGetCameraList 调用
*/
func (v VitataStatus) appsGetDeviceList(appName string, audienceList map[string]map[string]interface{}, deviceUserSetKey string) map[string]map[string]interface{} {
	_logger.Debug("appsGetDeviceList: appName = [%s] , deviceUserSetKey = [%s]", appName, deviceUserSetKey)
	deviceList := make(map[string]map[string]interface{})
	deviceUserSet, _ := Redis.Strings(redis.Do(SMEMBERS, deviceUserSetKey))
	if 0 != len(deviceUserSet) {
		for _, uid := range deviceUserSet {
			if _, found := audienceList[uid]; found {
				deviceList[uid] = audienceList[uid]
			} else {
				_logger.Warn("appsGetDeviceList key is [%s], uid[%s] doesn't exists.",
					deviceUserSetKey, uid)
			}
		}
	}
	_logger.Debug("deviceList: %s", serialize(deviceList))
	return deviceList
}

/*
   获取 当前麦克风用户列表
   此时需要传入 audienceList 参数, 因为micList 里 key uid 的属性是 participant
*/
func (v VitataStatus) appsGetMicList(appName string, audienceList map[string]map[string]interface{}) map[string]map[string]interface{} {
	micUserSetKey := VTT_MICUSERSET_PREFIX + appName
	return v.appsGetDeviceList(appName, audienceList, micUserSetKey)
}

/*
   获取 当前摄像头用户列表
   此时需要传入 audienceList 参数, 因为cameraList 里 key uid 的属性是 participant
*/
func (v VitataStatus) appsGetCameraList(appName string, audienceList map[string]map[string]interface{}) map[string]map[string]interface{} {
	cameraUserSetKey := VTT_CAMERAUSERSET_PREFIX + appName
	return v.appsGetDeviceList(appName, audienceList, cameraUserSetKey)
}

/*
   获取 当前禁言用户列表
   此时需要传入 audienceList 参数, 因为denyChatList 里 key uid 的属性是 participant
*/
func (v VitataStatus) appsGetDenyChatList(appName string, audienceList map[string]map[string]interface{}) map[string]map[string]interface{} {
	denyChatUserSetKey := VTT_DENYCHATUSERSET_PREFIX + appName
	denyChatList := make(map[string]map[string]interface{})
	denyChatUserSet, _ := Redis.Strings(redis.Do(SMEMBERS, denyChatUserSetKey))
	if 0 != len(denyChatUserSet) {
		for _, uid := range denyChatUserSet {
			if _, found := audienceList[uid]; found {
				denyChatList[uid] = audienceList[uid]
			} else {
				audience := map[string]interface{}{"uid": uid, "username": "", "role": 2, "imageUrl": ""}
				denyChatList[uid] = audience
			}
		}
	}
	_logger.Debug(serialize(denyChatList))
	return denyChatList
}

/*
   获取 当前语音禁言用户列表
   此时需要传入 audienceList 参数, 因为denyMList 里 key uid 的属性是 participant
*/
func (v VitataStatus) appsGetDenyMicList(appName string, audienceList map[string]map[string]interface{}) map[string]map[string]interface{} {
	denyMicUserSetKey := VTT_DENYMICUSERSET_PREFIX + appName
	denyMicList := make(map[string]map[string]interface{})
	denyMicUserSet, _ := Redis.Strings(redis.Do(SMEMBERS, denyMicUserSetKey))
	if 0 != len(denyMicUserSet) {
		for _, uid := range denyMicUserSet {
			if _, found := audienceList[uid]; found {
				denyMicList[uid] = audienceList[uid]
			} else {
				audience := map[string]interface{}{"uid": uid, "username": "", "role": 2, "imageUrl": ""}
				denyMicList[uid] = audience
			}
		}
	}
	return denyMicList
}

/*
  获取当前最近30条聊天信息
*/
func (v VitataStatus) appsGetChatMessages(appName string) []map[string]interface{} {
	chatMessages := make([]map[string]interface{}, 0)
	chatMsgKey := VTT_CHATMSG_PREFIX + appName
	chatMsgType, _ := Redis.String(redis.Do(TYPE, chatMsgKey))
	if chatMsgType == R_LIST_TYPE {
		chatMessageJsonList, _ := Redis.Strings(redis.Do(LRANGE, chatMsgKey, (-CHAT_MESSAGE_HISTORY_SIZE), -1))
		for _, chatMessageJson := range chatMessageJsonList {
			newJson, _ := simplejson.NewJson([]byte(chatMessageJson))
			if newJson == nil {
				_logger.Error("chatMessageJson [%s] is ERROR. ", chatMessageJson)
				continue
			}
			chatMessage, err := newJson.Map()
			if err == nil {
				chatMessages = append(chatMessages, chatMessage)
			}
		}
	} else if chatMsgType != R_NONE_TYPE {
		redis.Do(DEL, chatMsgKey)
	}
	return chatMessages
}

/*
   获取教室ppt信息（房间初始化时调用）
*/
func (v VitataStatus) appsGetPresentation(appName string) map[string]interface{} {
	presentation := make(map[string]interface{})
	presentKey := VTT_PRESENT_PREFIX + appName
	presentType, _ := Redis.String(redis.Do(TYPE, presentKey))
	if presentType == R_HASH_TYPE {
		reply, _ := Redis.Strings(redis.Do(HGETALL, presentKey))
		replyMap := list2Map(reply)
		presentation["id"] = replyMap["id"]
		presentation["slideCount"], _ = strconv.Atoi(replyMap["slideCount"])
		presentation["currentSlidePageNumber"], _ = strconv.Atoi(replyMap["currentSlidePageNumber"])
		presentation["uuid"] = replyMap["uuid"]
		presentation["name"] = replyMap["name"]
	} else if presentType != R_NONE_TYPE {
		redis.Do(DEL, presentKey)
	}
	return presentation
}

/*
  获取当前所有的划线数据
*/
func (v VitataStatus) appsGetPresentationLines(appName string) []map[string]interface{} {
	presentationLines := make([]map[string]interface{}, 0)
	presentationLinesKey := VTT_PRESENTATIONLINES_PREFIX + appName
	presentationLinesType, _ := Redis.String(redis.Do(TYPE, presentationLinesKey))
	if presentationLinesType == R_LIST_TYPE {
		presentationLinesJsonList, _ := Redis.Strings(redis.Do(LRANGE, presentationLinesKey, 0, -1))
		for _, presentationLineJson := range presentationLinesJsonList {
			newJson, _ := simplejson.NewJson([]byte(presentationLineJson))
			if newJson == nil {
				_logger.Error("presentationLineJson [%s] is ERROR. ", presentationLineJson)
				continue
			}
			presentationLine, err := newJson.Map()
			if err == nil {
				presentationLines = append(presentationLines, presentationLine)
			}
		}
	} else if presentationLinesType != R_NONE_TYPE {
		redis.Do(DEL, presentationLinesKey)
	}
	return presentationLines
}

/*
  获得PPT滚动位置
*/
func (v VitataStatus) appsGetPresentationScrollPos(appName string) (presentationScrollPos string) {
	presentationScrollPosKey := VTT_PRESENTATIONSCROLLPOS_PREFIX + appName
	presentationScrollPosType, _ := Redis.String(redis.Do(TYPE, presentationScrollPosKey))
	if presentationScrollPosType == R_STRING_TYPE {
		presentationScrollPos, _ = Redis.String(redis.Do(GET, presentationScrollPosKey))
	} else if presentationScrollPosType != R_NONE_TYPE {
		redis.Do(DEL, presentationScrollPosKey)
	}
	return
}

/*
  获取当前所有的写字事件
*/
func (v VitataStatus) appsGetPresentationTexts(appName string) []interface{} {
	presentationTexts := make([]interface{}, 0)
	presentationTextsKey := VTT_PRESENTATIONTEXTS_PREFIX + appName
	presentationTextsType, _ := Redis.String(redis.Do(TYPE, presentationTextsKey))
	if presentationTextsType == R_LIST_TYPE {
		presentationTextsJsonList, _ := Redis.Strings(redis.Do(LRANGE, presentationTextsKey, 0, -1))
		for _, presentationTextJson := range presentationTextsJsonList {
			newJson, _ := simplejson.NewJson([]byte(presentationTextJson))
			if newJson == nil {
				_logger.Error("presentationTextJson [%s] is ERROR. ", presentationTextJson)
				continue
			}
			presentationText, err := newJson.Array()
			if err == nil {
				presentationTexts = append(presentationTexts, presentationText)
			}
		}
	} else if presentationTextsType != R_NONE_TYPE {
		redis.Do(DEL, presentationTextsKey)
	}
	return presentationTexts
}

/*
  获得教室权限
  鼓掌 举手 公开聊天相关权限
*/
func (v VitataStatus) appsGetRoomActivityPolicy(appName string) string {
	roomActivityPolicyKey := VTT_ROOMACTIVITYPOLICY_PREFIX + appName
	roomActivityPolicyType, _ := Redis.String(redis.Do(TYPE, roomActivityPolicyKey))

	var roomActivityPolicy string
	if roomActivityPolicyType == R_STRING_TYPE {
		roomActivityPolicy, _ = Redis.String(redis.Do(GET, roomActivityPolicyKey))
	} else if roomActivityPolicyType != R_NONE_TYPE {
		redis.Do(DEL, roomActivityPolicyKey)
	}
	return roomActivityPolicy
}

/*
   获取教室通用状态
*/
func (v VitataStatus) appsGetGeneralStatus(appName string) map[string]interface{} {
	statusListKey := VTT_GENERALSTATUSLIST_PREFIX + appName
	statusList, _ := Redis.Strings(redis.Do(SMEMBERS, statusListKey))
	_logger.Info("%s", serialize(statusList))
	resultMap := make(map[string]interface{})
	for _, value := range statusList {
		currentKey := VTT_GENERALSTATUS_PREFIX + appName + ":" + value
		keyType, _ := Redis.String(redis.Do(TYPE, currentKey))
		if keyType == R_HASH_TYPE {
			reply, _ := Redis.Strings(redis.Do(HGETALL, currentKey))
			replyMap := list2Map(reply)
			resultMap[value] = replyMap
		} else if keyType != R_NONE_TYPE {
			redis.Do(DEL, currentKey)
		}
	}

	return resultMap
}

/*
  获得当前教室的初始化状态
  audienceList        用户列表            map  结构
  micList             麦克风用户列表      map  结构
  cameraList          摄像头用户列表      map  结构
  chatMessages        聊天信息(最近30条)  list 结构
  presentation        课件状态            map  结构
  roomActivityPolicy  教室权限            map  结构
*/
func (v *VitataStatus) AppsGetGlobalRoomStatus(sockIdentity, cliIdentity, appName string) common.CustomMap {
	_logger.Debug("getGlobalRoomStatus")
	currentVer := v.appsGetCurrentVersion(appName)
	if currentVer < 0 {
		currentVer = 0
		v.ResetRoom(appName)
	}
	var audienceList map[string]map[string]interface{} //用户列表
	var micList map[string]map[string]interface{}      //麦克风列表
	var cameraList map[string]map[string]interface{}   //摄像头列表
	var denyChatList map[string]map[string]interface{} //禁言列表
	var denyMicList map[string]map[string]interface{}  //禁言列表
	var chatMessages []map[string]interface{}          //聊天列表
	var presentation map[string]interface{}            //PPT信息
	var presentationLines []map[string]interface{}     //PPT划线信息
	var presentationTexts []interface{}                //PPT写字信息
	var presentationScrollPos string                   //PPT滚动位置
	var roomActivityPolicyContent string               //房间策略
	var generalStatus map[string]interface{}           //房间策略
	var countMap map[string]interface{}                //学生人数map
	if currentVer >= 0 {
		audienceList = v.appsGetAudienceList(appName)
		micList = v.appsGetMicList(appName, audienceList)
		cameraList = v.appsGetCameraList(appName, audienceList)
		denyChatList = v.appsGetDenyChatList(appName, audienceList)
		denyMicList = v.appsGetDenyMicList(appName, audienceList)
		chatMessages = v.appsGetChatMessages(appName)
		presentation = v.appsGetPresentation(appName)
		presentationLines = v.appsGetPresentationLines(appName)
		presentationScrollPos = v.appsGetPresentationScrollPos(appName)
		presentationTexts = v.appsGetPresentationTexts(appName)
		roomActivityPolicyContent = v.appsGetRoomActivityPolicy(appName)
		generalStatus = v.appsGetGeneralStatus(appName)
		countMap = v.appsGetUserCount(appName)
	}
	/*
		need to fix
		//FIX ME
	*/
	roomActivityPolicy := make(map[string]interface{})
	if roomActivityPolicyContent != "" {
		if roomActivityPolicyJson, err := simplejson.NewJson([]byte(roomActivityPolicyContent)); err == nil {
			roomActivityPolicy = roomActivityPolicyJson.MustMap()
		}
	}

	globalRoomStatus := common.CustomMap{
		"version":                         currentVer,
		KEY_STATUS_AUDIENCE_LIST:          audienceList,
		KEY_STATUS_COUNT_MAP:              countMap,
		KEY_STATUS_MIC_LIST:               micList,
		KEY_STATUS_CAMERA_LIST:            cameraList,
		KEY_STATUS_DENYCHAT_LIST:          denyChatList,
		KEY_STATUS_DENYMIC_LIST:           denyMicList,
		KEY_STATUS_CHAT_MESSAGES:          chatMessages,
		KEY_STATUS_PRESENTATION:           presentation,
		KEY_STATUS_ROOM_ACTIVITY_POLICY:   roomActivityPolicy,
		KEY_STATUS_PRESENTATION_SCROLLPOS: presentationScrollPos,
		KEY_STATUS_PRESENTATION_LINES:     presentationLines,
		KEY_STATUS_PRESENTATION_TEXTS:     presentationTexts,
		KEY_STATUS_GENERAL_STATUS:         generalStatus,
	}
	_logger.Debug("globalRoomStatus: %s.", serialize(globalRoomStatus))
	cliIdentityInfo := strings.SplitN(cliIdentity, ":", -1)
	if len(cliIdentityInfo) == 3 {
		instanceIP := cliIdentityInfo[1]
		instancePID := cliIdentityInfo[2]
		dealerIdKey := VTT_DEALERID_PREFIX + instanceIP + ":" + instancePID + ":" + appName
		redis.Do(MULTI)
		redis.Send(SET, dealerIdKey, sockIdentity)
		redis.Send(EXPIRE, dealerIdKey, EXPIRE_TIME)
		redis.Do(EXEC)
	} else {
		_logger.Error("appsGetGlobalRoomStatus cliIdentity may be error: %s.", cliIdentity)
	}
	return globalRoomStatus
}

// 单例
func NewVitataStatus(workerId, redisServerAddress string, redisServerPort string) *VitataStatus {
	_mutex.Lock()
	defer _mutex.Unlock()
	//if _vitataStatus == nil {
	_vitataStatus = new(VitataStatus)
	//_vitataStatus.aliveInstanceMap = _vitataStatus.getInstanceMapByWorkerId(workerId)
	_vitataStatus.lastTimestamp = time.Now()
	_vitataStatus.curTimestamp = time.Now()
	_vitataStatus.redisAddress = redisServerAddress + ":" + redisServerPort
	_vitataStatus.workerId = workerId
	//}
	return _vitataStatus
}

var curClock time.Time
var preClock time.Time

func (v *VitataStatus) BackgroundTask() {
	curClock = time.Now()
	//每个intervalTime时间输出所有收发消息（默认间隔是60s（1min））
	if curClock.Sub(preClock).Seconds() >= 5 {
		preClock = curClock
		roomIdList, _ := Redis.Strings(redis.Do(SMEMBERS, VTT_ROOMIDLIST_KEY))
		timestamp := time.Now().Unix() * 1000
		for _, roomId := range roomIdList {
			appName := "apps/" + roomId
			args := v.appsGetUserCount(appName)

			event := common.CustomMap{
				"type":         "BroadcastEvent",
				"roomId":       roomId,
				"functionName": CLI_SIMPLE_COUNT,
				"timestamp":    timestamp,
				"args":         args,
			}

			eventMsgToClient := common.CustomMap{
				"event":   event,
				"appName": appName,
			}

			sockIdentity := "EVENT:" + appName
			sendBroadcastMsgCallBackFunc(sockIdentity, appName, eventMsgToClient)
		}
	}

}

//设置日志文件指针
func SetLogger(logger l4g.Logger) {
	_logger = logger
}

func printWarn(warnMsg string) {
	if _logger != nil {
		_logger.Warn(warnMsg)
	} else {
		fmt.Println(warnMsg)
	}
}

var sendRelpyEventMsgCallBackFunc func(string, string, common.CustomMap)
var sendBroadcastMsgCallBackFunc func(string, string, common.CustomMap)

func SetCallBackFunc(initSendExternalMsgCallBackFunc func(string, string, common.CustomMap), initSendBroadcastMsgCallBackFunc func(string, string, common.CustomMap)) {
	sendRelpyEventMsgCallBackFunc = initSendExternalMsgCallBackFunc
	sendBroadcastMsgCallBackFunc = initSendBroadcastMsgCallBackFunc
}

func InitRedis(redisServerAddress, redisServerPort string) bool {
	redisAddress = redisServerAddress + ":" + redisServerPort
	if redis == nil {
		_logger.Debug("redis is nil, getRedis()")
		redis = getRedis()
		if redis == nil {
			_logger.Error("Can not connect to the redis")
			return false
		} else {
			return true
		}
	} else {
		return true
	}
}
