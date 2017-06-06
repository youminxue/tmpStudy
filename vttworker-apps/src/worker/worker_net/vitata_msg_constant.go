//  Vitata
//
//  @file     vitata_msg_constant.go
//  @author   cf <feng.chen@duobei.com>
//  @date     Wed Apr 23 10:12:00 2014
//  @version  golang 1.2.1
//  @brief    Rewrite the vitata_msg_constant.lua in luajit 2.0.2 and lua 5.1
//
//  http://www.duobei.com
//  http://www.wangxiaotong.com
//  http://www.jiangzuotong.com
//

package worker_net

/*

//    dealer:
msg head prefix:       REQ
                    [client] dealer -> router [proxy_apps] back router -> dealer [worker]  -\
                                                                                            |
                    [client] dealer <- router [proxy_apps] back router <- dealer [worker] <-/
msg head prefix:                                                                   REP
*/

const (
	REQ_MSG_TYPE_PREFIX          = "REQ"
	REQ_MSG_TYPE_INIT_PREFIX     = REQ_MSG_TYPE_PREFIX + ":INIT:"
	REQ_MSG_TYPE_EVENT_PREFIX    = REQ_MSG_TYPE_PREFIX + ":EVENT:"
	REQ_MSG_TYPE_PING_PREFIX     = REQ_MSG_TYPE_PREFIX + ":PING:"     // + IP:PID
	REQ_MSG_TYPE_REGAPP_PREFIX   = REQ_MSG_TYPE_PREFIX + ":REGAPP:"   // unuse
	REQ_MSG_TYPE_UNREGAPP_PREFIX = REQ_MSG_TYPE_PREFIX + ":UNREGAPP:" // + IP:PID:appName
	REQ_MSG_TYPE_MONITOR_PREFIX  = REQ_MSG_TYPE_PREFIX + ":MONITOR:"  // send to Monitor Center
)

const (
	REP_MSG_TYPE_PREFIX       = "REP"
	REP_MSG_TYPE_INIT_PREFIX  = REP_MSG_TYPE_PREFIX + ":INIT:"
	REP_MSG_TYPE_EVENT_PREFIX = REP_MSG_TYPE_PREFIX + ":EVENT:"
)

/*
    channel:
    1. echo: notify pub sub ready: channel name is unique!!
       channel name is ECHO:$appName:$ip:$pid

msg header prefix:   PUB                             SUB
                   [client] pub -> xsub [proxy_apps] xpub -> sub [client]

    2. event: pub/sub msg
       channel name is EVENT:$appName

simple:
msg header prefix:   PUB                             SUB
                   [client] pub -> xsub [proxy_apps] xpub -> sub [client]

update:
msg header prefix:   PUB
                   [client] pub -> xsub [proxy_apps] router -> dealer [worker] -\
                                                                                |
                   [client] sub <- xpub [proxy_apps] router <- dealer [worker] -/
msg header prefix:                                              SUB

*/
const (
	PUB_MSG_TYPE_PREFIX        = "PUB"
	PUB_MSG_TYPE_ECHO          = PUB_MSG_TYPE_PREFIX + ":ECHO"
	PUB_MSG_TYPE_SIMPLE_PREFIX = PUB_MSG_TYPE_PREFIX + ":SIMPLE:"
	PUB_MSG_TYPE_UPDATE_PREFIX = PUB_MSG_TYPE_PREFIX + ":UPDATE:"

	SUB_MSG_TYPE_PREFIX        = "SUB"
	SUB_MSG_TYPE_ECHO          = SUB_MSG_TYPE_PREFIX + ":ECHO"
	SUB_MSG_TYPE_SIMPLE_PREFIX = SUB_MSG_TYPE_PREFIX + ":SIMPLE:"
	SUB_MSG_TYPE_UPDATE_PREFIX = SUB_MSG_TYPE_PREFIX + ":UPDATE:"
)

//use in proxy_apps
const (
	CONN_STATE_NOTIFY_PREFIX  = "CONN_NOTIFY"
	CONN_STATE_CONNECT_PREFIX = CONN_STATE_NOTIFY_PREFIX + ":CONNECT:"
	CONN_STATE_DISCONN_PREFIX = CONN_STATE_NOTIFY_PREFIX + ":DISCONN:"
)

/*

   msg     WORKER:ALIVE
            [worker]    dealer  ->  router [proxy_apps]


   proxy_apps has:
     workerAliveMap     : table  | [workerId] -> timestamp (last alive timestamp) roomCount
     appId_worker_map   : table  | [appId] -> workerId
     recommendWorkerId  : string | whose roomCount may has the minimal (why is may, read Note 1, proxy_apps scan every 2 min)

   Note:
     1) every 1 min worker send the keep alive msg to proxy_apps,
        then workerAliveMap[workerId] set current timestamp:
        if workerAliveMap[workerId] is nil then
           workerAliveMap[workerId] = {timestamp = currentTimestamp, roomCount = 0}
        else
           workerAliveMap[workerId].timestamp = currentTimestamp

        every 2 min proxy_apps scan workerAliveMap
        if currentTimestamp - 2 min > workerAliveMap[someId].timestamp then
                workerGroup[someId] = nil

        find the workerId who is alive and has the minimal roomCount, then
                recommendWorkerId = workerId

     2) client(appId) send a msg, and proxy_apps need to find the worker to send:
        follow as such rules:
        .1 check appId_worker_map[appId], if has workerId, go to step 2, else go to step 3

        .2 check workerAliveMap[workerId]
           if workerAliveMap[workerId] is nil then -- proxy_apps check has clear it
               appId_worker_map[appId] = recommendWorkerId
               (
                 Q: why recommendWorkerId is legal
                 A: proxy_apps every 2 min check, and reset recommendWorkerId, we assume that it's always wright
               )
           go to step 4

        .3 appId_worker_map[appId] = recommendWorkerId
        .4 appId send msg to its workerId


*/
const (
	STATUS_WORKER_KEEPALIVE = "WORKERALIVE"
)
