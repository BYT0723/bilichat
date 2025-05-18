package biliclient

type handShakeInfo struct {
	UID      uint32 `json:"uid"`
	Roomid   uint32 `json:"roomid"`
	Protover uint8  `json:"protover"`
	Buvid    string `json:"buvid"`
	Platform string `json:"platform"`
	Type     uint8  `json:"type"`
	Key      string `json:"key"`
}
