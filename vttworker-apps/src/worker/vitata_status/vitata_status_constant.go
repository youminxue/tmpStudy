//  Vitata
//
//  @file     vitata_status_constant.go
//  @author   cf <feng.chen@duobei.com>
//  @date     Wed Apr 23 10:12:00 2014
//  @version  golang 1.2.1
//  @brief    Rewrite the vitata_status_constant.lua in luajit 2.0.2 and lua 5.1
//
//  http://www.duobei.com
//  http://www.wangxiaotong.com
//  http://www.jiangzuotong.com
//

package vitata_status

const (
	VTT_PREFIX = "VTT"

	VTT_ROOMIDLIST_KEY         = VTT_PREFIX + ":ROOMIDLIST"
	VTT_VERSION_PREFIX         = VTT_PREFIX + ":VERSION:"         // + appName           string (set get)
	VTT_STUSET_PREFIX          = VTT_PREFIX + ":STUSET:"          // + appName      set  uid
	VTT_USERSET_PREFIX         = VTT_PREFIX + ":USERSET:"         // + appName      set  uid
	VTT_HST_USERSET_PREFIX     = VTT_PREFIX + ":HSTUSERSET:"      // + appName     历史用户列表 set  uid
	VTT_MICUSERSET_PREFIX      = VTT_PREFIX + ":MICUSERSET:"      // + appName set uid  micList
	VTT_CAMERAUSERSET_PREFIX   = VTT_PREFIX + ":CAMERAUSERSET:"   // + appName set uid  camList
	VTT_DENYCHATUSERSET_PREFIX = VTT_PREFIX + ":DENYCHATUSERSET:" // appName set uid  denyUserChatList
	VTT_DENYMICUSERSET_PREFIX  = VTT_PREFIX + ":DENYMICUSERSET:"  // appName set uid  denyUserMicList
	VTT_USER_PREFIX            = VTT_PREFIX + ":USER:"            // + appName:uid  hash
	VTT_INSTANCE_PREFIX        = VTT_PREFIX + ":INSTANCE:"        // + ip:pid     set  appName
	VTT_INSTANCEAPP_PREFIX     = VTT_PREFIX + ":INSTANCEAPP:"     // + ip:pid:appName set  uid
	VTT_DEALERID_PREFIX        = VTT_PREFIX + ":DEALERID:"        // + ip:pid:appName   string dealerId(sockIdentity)

	VTT_CHATMSG_PREFIX               = VTT_PREFIX + ":CHATMSG:"               // + appName           list
	VTT_PRESENT_PREFIX               = VTT_PREFIX + ":PRESENT:"               // + appName           hash
	VTT_PRESENTATIONLINES_PREFIX     = VTT_PREFIX + ":PRESENTATIONLINES:"     // + appName           list
	VTT_PRESENTATIONSCROLLPOS_PREFIX = VTT_PREFIX + ":PRESENTATIONSCROLLPOS:" // + appName           string
	VTT_PRESENTATIONTEXTS_PREFIX     = VTT_PREFIX + ":PRESENTATIONTEXTS:"     // + appName           list
	VTT_OPT_PREFIX                   = VTT_PREFIX + ":OPT:"                   // + appName           list
	VTT_ALIVEINFO_PREFIX             = VTT_PREFIX + ":ALIVEINFO:"             // + ip:pid       {workerId:workerId, timestamp:timestamp} string (set get)
	VTT_WORKER_INSTANCESET_PREFIX    = VTT_PREFIX + ":WORKER:"                // + workerId  set ip:pid

	VTT_ROOMACTIVITYPOLICY_PREFIX = VTT_PREFIX + ":ROOMACTIVITYPOLICY:" // + appName           hash
	VTT_GENERALSTATUS_PREFIX      = VTT_PREFIX + ":GENERALSTATUS:"      // + appName           hash
	VTT_GENERALSTATUSLIST_PREFIX  = VTT_PREFIX + ":GENERALSTATUSLIST:"  // + appName           set

	// prefix len
	VTT_VERSION_PREFIX_LEN            = 12
	VTT_USERSET_PREFIX_LEN            = 12
	VTT_MICUSERSET_PREFIX_LEN         = 15
	VTT_CAMERAUSERSET_PREFIX_LEN      = 18
	VTT_DENYCHATUSERSET_PREFIX_LEN    = 20
	VTT_USER_PREFIX_LEN               = 9
	VTT_INSTANCE_PREFIX_LEN           = 13
	VTT_INSTANCEAPP_PREFIX_LEN        = 16
	VTT_CHATMSG_PREFIX_LEN            = 12
	VTT_PRESENT_PREFIX_LEN            = 12
	VTT_PRESENTATIONLINES_PREFIX_LEN  = 22
	VTT_OPT_PREFIX_LEN                = 8
	VTT_ALIVEINFO_PREFIX_LEN          = 14
	VTT_WORKER_INSTANCESET_PREFIX_LEN = 11 // + workerId  set ip:pid

	VTT_ROOMACTIVITYPOLICY_PREFIX_LEN = 23
)

const (
	KEY_STATUS_VERSION       = "VERSION"
	KEY_STATUS_AUDIENCE_LIST = "AUDIENCELIST"
	KEY_STATUS_COUNT_MAP     = "COUNTMAP"
	KEY_STATUS_MIC_LIST      = "MICLIST"
	KEY_STATUS_CAMERA_LIST   = "CAMERALIST"
	KEY_STATUS_DENYCHAT_LIST = "DENYCHATLIST"
	KEY_STATUS_DENYMIC_LIST  = "DENYMICLIST"
	KEY_STATUS_TEACHER_INFO  = "TEACHERINFO"

	KEY_STATUS_CHAT_MESSAGES          = "CHATMESSAGES"
	KEY_STATUS_PRESENTATION           = "PRESENTATION"
	KEY_STATUS_PRESENTATION_LINES     = "PRESENTATIONLINES"
	KEY_STATUS_PRESENTATION_TEXTS     = "PRESENTATIONTEXTS"
	KEY_STATUS_PRESENTATION_SCROLLPOS = "PRESENTATIONSCROLLPOS"

	KEY_STATUS_ROOM_ACTIVITY_POLICY = "ROOMACTIVITYPOLICY"
	KEY_STATUS_GENERAL_STATUS       = "GENERALSTATUS"

	CLI_SENDTOCLIENT = "clientSendToClient"

	CLI_UPDATE_NO_FUNC            = "NOFUNC"
	CLI_UPDATE_USER_JOIN_FUNC     = "clientOnline"
	CLI_UPDATE_USER_LEFT_FUNC     = "clientOffline"
	CLI_UPDATE_USERS_LEFT_FUNC    = "clientsOffline"
	CLI_UPDATE_CHAT_FUNC          = "clientPublicChat"
	CLI_UPDATE_MIC_ON_AIR_FUNC    = "clientMicOnAir"
	CLI_UPDATE_MIC_OFF_FUNC       = "clientMicOff"
	CLI_UPDATE_CAMERA_ON_AIR_FUNC = "clientCameraOnAir"
	CLI_UPDATE_CAMERA_OFF_FUNC    = "clientCameraOff"

	CLI_UPDATE_ALLOWCHAT_UID_FUNC = "clientAllowChatByUid"
	CLI_UPDATE_DENYCHAT_UID_FUNC  = "clientDenyChatByUid"
	CLI_UPDATE_ALLOWMIC_UID_FUNC  = "clientAllowMicByUid"
	CLI_UPDATE_DENYMIC_UID_FUNC   = "clientDenyMicByUid"

	CLI_UPDATE_PRESENTATION_SLIDECHANGE_FUNC = "presentationSlideChanged"
	CLI_UPDATE_PRESENTATION_CHANGE_FUNC      = "presentationChanged"
	CLI_UPDATE_PRESENTATION_DRAWLINE_FUNC    = "presentationDrawLine"
	CLI_UPDATE_PRESENTATION_DRAWCLEAN_FUNC   = "presentationDrawClean"
	CLI_UPDATE_PRESENTATION_SLIDESCROLL_FUNC = "presentationSlideScroll"
	CLI_UPDATE_PRESENTATION_DRAWTEXT_FUNC    = "presentationDrawText"
	CLI_UPDATE_STATUS_SET_FUNC               = "clientStatusSet"
	CLI_UPDATE_STATUS_CLEAN_FUNC             = "clientStatusClean"

	CLI_SIMPLE_BROADCAST       = "clientBroadcast"
	CLI_SIMPLE_BROADCAST_EVENT = "clientBroadcastEvent"

	CLI_SIMPLE_RAISE_HAND = "clientRaiseHand"
	CLI_SIMPLE_DOWN_HAND  = "clientDownHand"
	CLI_SIMPLE_APPLAUD    = "clientApplaud"
	CLI_SIMPLE_COUNT      = "clientCount"

	CLI_SIMPLE_MIC_REQ_FUNC    = "clientMicReq"
	CLI_SIMPLE_CAMERA_REQ_FUNC = "clientCameraReq"

	CLI_SIMPLE_PRESENTATION_MOUSEMOVE   = "presentationMouseMove"
	CLI_SIMPLE_PRESENTATION_DRAWLINE    = "presentationDrawLine"
	CLI_SIMPLE_PRESENTATION_DRAWTEXT    = "presentationDrawText"
	CLI_SIMPLE_PRESENTATION_DRAWCLEAN   = "presentationDrawClean"
	CLI_SIMPLE_PRESENTATION_SLIDESCROLL = "presentationSlideScroll"

	MC_UPDATE_ROOM_ACTIVITY_POLICY_CONTROL_FUNC = "controlRoomActivityCallback"

	MIC_STATUS_OFF    = 0
	MIC_STATUS_ON_AIR = 2

	MIC_STATUS_PUBLISHING   = 1 // for nebula
	MIC_STATUS_UNPUBLISHING = 3 // for nebula

	CAMERA_STATUS_OFF    = 0
	CAMERA_STATUS_ON_AIR = 2

	CAMERA_STATUS_PUBLISHING   = 1 // for nebula
	CAMERA_STATUS_UNPUBLISHING = 3 // for nebula
)
