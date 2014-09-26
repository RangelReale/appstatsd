package infohttp

// Response header
type InfoResponse struct {
	ErrorCode    int32  `json:"error_code"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Must always be a struct or map, CANNOT be array
	Data interface{} `json:"data"`
}

type InfoResultRaw struct {
	List interface{} `json:"list"`
}
