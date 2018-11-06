package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	apiTimeout = 15 * time.Second
)

type DatadogMonitor struct {
	ID      int64    `json:"id,omitempty"`
	Type    string   `json:"type,omitempty"`
	Query   string   `json:"query,omitempty"`
	Message string   `json:"message,omitempty"`
	Name    string   `json:"name,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Options *Options `json:"options,omitempty"`
}

type Options struct {
	NotifyAudit       bool   `json:"notify_audit,omitempty"`
	Locked            bool   `json:"locked,omitempty"`
	NoDataTimeFrame   int64  `json:"no_data_timeframe,omitempty"`
	NewHostDelay      int64  `json:"new_host_delay,omitempty"`
	RequireFullWindow bool   `json:"require_full_window,omitempty"`
	NotifyNoData      bool   `json:"notify_no_data,omitempty"`
	TimeoutH          int64  `json:"timeout_h,omitempty"`
	RenotifyInterval  int64  `json:"renotify_interval,omitempty"`
	EscalationMessage string `json:"escalation_message,omitempty"`
	IncludeTags       bool   `json:"include_tags,omitempty"`
}

type ErrNotFound struct {
	Status int
	Msg    string
}

func Save(mon *DatadogMonitor) (int64, error) {
	baseURL, apiKey, appKey := getDdParams()
	url := fmt.Sprintf("%s/api/v1/monitor?api_key=%s&application_key=%s", baseURL, apiKey, appKey)
	verb := "POST"

	if mon.ID != 0 {
		url = fmt.Sprintf("%s/api/v1/monitor/%d?api_key=%s&application_key=%s", baseURL, mon.ID, apiKey, appKey)
		verb = "PUT"
	}

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(mon)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest(verb, url, buf)
	req.Header.Set("Content-type", "application/json")

	client := &http.Client{
		Timeout: apiTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf(resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	m := new(DatadogMonitor)
	err = json.Unmarshal(body, &m)
	if err != nil {
		return 0, err
	}

	return m.ID, nil
}

func Delete(id int64) error {
	baseURL, apiKey, appKey := getDdParams()
	url := fmt.Sprintf("%s/api/v1/monitor/%d?api_key=%s&application_key=%s", baseURL, id, apiKey, appKey)

	req, err := http.NewRequest("DELETE", url, nil)
	client := &http.Client{
		Timeout: apiTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf(resp.Status)
	}

	return nil
}

func Get(id int64) (*DatadogMonitor, error) {
	baseURL, apiKey, appKey := getDdParams()
	url := fmt.Sprintf("%s/api/v1/monitor/%d?api_key=%s&application_key=%s", baseURL, id, apiKey, appKey)
	resp, err := http.Get(url)
	if err != nil {
		return &DatadogMonitor{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &DatadogMonitor{}, &ErrNotFound{Msg: resp.Status}
	}

	if resp.StatusCode >= 400 {
		return &DatadogMonitor{}, fmt.Errorf(resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &DatadogMonitor{}, err
	}

	m := new(DatadogMonitor)
	err = json.Unmarshal(body, &m)
	if err != nil {
		return m, err
	}

	return m, nil
}

func (h *ErrNotFound) Error() string {
	return h.Msg
}

// XXX should get that from some sort of config or cli opts
func getDdParams() (string, string, string) {
	return os.Getenv("DD_URL"),
		os.Getenv("API_KEY"),
		os.Getenv("APP_KEY")
}
