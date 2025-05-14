package biliclient

type receivedInfo struct {
	Cmd        string                 `json:"cmd"`
	Data       map[string]interface{} `json:"data"`
	Info       []interface{}          `json:"info"`
	Full       map[string]interface{} `json:"full"`
	Half       map[string]interface{} `json:"half"`
	Side       map[string]interface{} `json:"side"`
	RoomID     uint32                 `json:"roomid"`
	RealRoomID uint32                 `json:"real_roomid"`
	MsgCommon  string                 `json:"msg_common"`
	MsgSelf    string                 `json:"msg_self"`
	LinkUrl    string                 `json:"link_url"`
	MsgType    string                 `json:"msg_type"`
	ShieldUID  string                 `json:"shield_uid"`
	BusinessID string                 `json:"business_id"`
	Scatter    map[string]interface{} `json:"scatter"`
}

type handShakeInfo struct {
	UID      uint32 `json:"uid"`
	Roomid   uint32 `json:"roomid"`
	Protover uint8  `json:"protover"`
	Buvid    string `json:"buvid"`
	Platform string `json:"platform"`
	Type     uint8  `json:"type"`
	Key      string `json:"key"`
}
