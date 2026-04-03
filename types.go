package mindp

import "encoding/json"

type cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain,omitempty"`
	Path     string  `json:"path,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"`
}

type evaluateResult struct {
	Result struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value,omitempty"`
	} `json:"result"`
}

type wsVersionInfo struct {
	Browser              string `json:"Browser"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type createTargetResult struct {
	TargetID string `json:"targetId"`
}

type attachTargetResult struct {
	SessionID string `json:"sessionId"`
}

type addScriptResult struct {
	Identifier string `json:"identifier"`
}

type screenshotResult struct {
	Data string `json:"data"`
}

type nodeResult struct {
	Root struct {
		NodeID int64 `json:"nodeId"`
	} `json:"root"`
}

type querySelectorResult struct {
	NodeID int64 `json:"nodeId"`
}

type domBoxModel struct {
	Model struct {
		Content []float64 `json:"content"`
	} `json:"model"`
}

type storageState struct {
	Cookies      []Cookie          `json:"cookies"`
	LocalStorage map[string]string `json:"localStorage,omitempty"`
	SessionStore map[string]string `json:"sessionStorage,omitempty"`
	Origin       string            `json:"origin,omitempty"`
}

type RequestEvent struct {
	Time      int64
	RequestID string
	URL       string
	Method    string
	Headers   map[string]any
}

type ResponseEvent struct {
	Time      int64
	RequestID string
	URL       string
	Status    int
	MIMEType  string
	Headers   map[string]any
}
