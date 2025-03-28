package subscribefilter

import (
	"fmt"
	"strings"
)

const (
	AdminCode         string = "0"
	RegionCode        string = "1"
	ExternalGovCode   string = "2"
	EnterpriseCode    string = "3"
	NaturalPersonCode string = "4"

	RootCode = "00"
)

// IdentityCode 订阅者身份编码
/**
总局编码为   0 00 00 00 xxxx 这样的11位表示
省局编码为   1 xx xx xx xxxx 这样的11位表示，如 1 13 01 02
外部机构为   2 xx xx xx xx xxxxxxxxxx 这样的19位表示
企业编码为   3 xx xx xx xx xxxxxxxxxx 这样的19位表示
自然人编码   4 xx xx xx xxxxxxxxxxxx 这样的19位表示
*/
type IdentityCode struct {
	CodeType     string // 第1位, [0,1)
	ProvinceCode string // 第2，3位, [1,3)
	CityCode     string // 第4，5位, [3,5)
	DistrictCode string // 第6，7位, [5,7)
	Extension    string // 11位编码时4位，19位编码时2 + 10位
	Raw          string // 原始编码字符串
}

// GetIdentityCode 转换身份编码
func GetIdentityCode(rawId string) (*IdentityCode, error) {
	id := strings.ReplaceAll(rawId, " ", "")
	length := len(id)
	// 位数判断
	if length != 11 && length != 19 {
		return nil, fmt.Errorf("unexpected id length, id: [%v]", rawId)
	}
	ic := &IdentityCode{
		Raw:          id,
		CodeType:     id[0:1],
		ProvinceCode: id[1:3],
		CityCode:     id[3:5],
		DistrictCode: id[5:7],
	}
	if length == 11 {
		ic.Extension = id[7:11]
	} else {
		ic.Extension = id[7:19]
	}

	if !ic.verify() {
		return nil, fmt.Errorf("unexpected id format, id: [%v]", rawId)
	}
	return ic, nil
}

func (ic *IdentityCode) Match(participantIdCode *IdentityCode) bool {
	if ic == nil {
		return false
	}
	// AdminCode 要订阅所有内容，即使是alias无关的内容
	if ic.CodeType == AdminCode {
		return true
	}
	if participantIdCode == nil {
		return false
	}
	switch ic.CodeType {
	case RegionCode:
		// 类型 "1" 为省局、市局或区局
		// 如果市和区均为 "00"，视为省局，匹配时只比较省编码
		if ic.CityCode == RootCode && ic.DistrictCode == RootCode {
			return ic.ProvinceCode == participantIdCode.ProvinceCode
		}
		// 如果区为 "00"，视为市局，匹配省和市
		if ic.DistrictCode == RootCode {
			return ic.ProvinceCode == participantIdCode.ProvinceCode &&
				ic.CityCode == participantIdCode.CityCode
		}
		// 否则视为区局，需要匹配省、市、区
		return ic.ProvinceCode == participantIdCode.ProvinceCode &&
			ic.CityCode == participantIdCode.CityCode &&
			ic.DistrictCode == participantIdCode.DistrictCode
	case ExternalGovCode, EnterpriseCode, NaturalPersonCode:
		// 对于外部政府部门、企业、自然人，要求全码匹配
		return ic.Raw == participantIdCode.Raw
	default:
		// 其他类型不匹配
		return false
	}
	return true
}

func (ic *IdentityCode) verify() bool {
	if ic == nil {
		return false
	}
	if !IsValidCodeType(ic.CodeType) {
		return false
	}
	if ic.CodeType == AdminCode {
		return ic.ProvinceCode == RootCode &&
			ic.CityCode == RootCode &&
			ic.DistrictCode == RootCode
	}
	if ic.ProvinceCode == RootCode {
		return false
	}
	return true
}

func IsValidCodeType(codeType string) bool {
	switch codeType {
	case AdminCode, RegionCode, ExternalGovCode, EnterpriseCode, NaturalPersonCode:
		return true
	default:
		return false
	}
}
