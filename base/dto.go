package base

type AuthDto struct {
	Remote string `json:"remote"`
	Port   int    `json:"port"`
	User   string `json:"user"`
	Passwd string `json:"passwd"`
	Tid    string `json:"tid"`
}

type AuthRsp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
