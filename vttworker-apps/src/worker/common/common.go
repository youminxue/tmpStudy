//  Vitata
//
//  @file     vitata_status_constant.go
//  @author   cf <feng.chen@duobei.com>
//  @date     Wed Apr 23 10:12:00 2014
//  @version  golang 1.2.1
//
//  http://www.duobei.com
//  http://www.wangxiaotong.com
//  http://www.jiangzuotong.com
//

package common

import (
	"reflect"
	"strconv"
)

type CustomMap map[string]interface{}

type Participant struct {
	uid        string
	protocolId int
	PID        string
	IP         string
	role       int
	imageUrl   string
	username   string
}

func (p *Participant) Map2Struct(inputMap map[string]string) bool {
	uid, found1 := inputMap["uid"]
	protocolId, found2 := inputMap["protocolId"]
	PID, found3 := inputMap["PID"]
	IP, found4 := inputMap["IP"]
	role, found5 := inputMap["role"]
	imageUrl, found6 := inputMap["imageUrl"]
	username, found7 := inputMap["username"]

	if found1 && found2 && found3 && found4 && found5 && found6 && found7 {
		p.uid = uid
		p.protocolId, _ = strconv.Atoi(protocolId)
		p.PID = PID
		p.IP = IP
		p.role, _ = strconv.Atoi(role)
		p.imageUrl = imageUrl
		p.username = username
		return true
	}
	return false
}
func (participant Participant) struct2Map() map[string]interface{} {
	resultMap := make(map[string]interface{})
	vType := reflect.TypeOf(participant)
	vValue := reflect.ValueOf(participant)
	for i := 0; i < vType.NumField(); i++ {
		field := vType.Field(i)
		if field.Name == "protocolId" || field.Name == "role" {
			resultMap[field.Name] = vValue.FieldByName(field.Name).Int()
		} else {
			resultMap[field.Name] = vValue.FieldByName(field.Name).String()
		}
	}
	return resultMap
}
