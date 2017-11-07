package api

type InitRequest struct {
	AppName string `json:"appName"`
}

type InitResponse struct {
	AppName      string                 `json:"appName"`
	Components   map[string]interface{} `json:"components"`
	Environments map[string]interface{} `json:"environments"`
}

type ShowRequest struct {
	AppName      string                 `json:"appName"`
	Components   map[string]interface{} `json:"components"`
	Environments map[string]interface{} `json:"environments"`
}

type ShowResponse struct {
	Components map[string]interface{} `json:"components"`
}

type GenerateRequest struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}

type GenerateResponse struct {
	Components map[string]interface{} `json:"components"`
}
