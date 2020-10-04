package valapiservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

// ValAPIService valorantに関するriot APIを利用するためのクラス
type ValAPIService struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type matchList struct {
	puuid   string
	history []match
}

type match struct {
	MatchID             string `json:"matchId"`
	GameStartTimeMillis int    `json:"gameStartTimeMillis"`
	TeamID              string `json:"timeId"`
}

type account struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// NewValAPIService コンストラクタ
func NewValAPIService(apiKey, baseURL string) *ValAPIService {
	return &ValAPIService{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (vas *ValAPIService) getRequestHeader() map[string]string {
	return map[string]string{
		"X-Riot-Token": vas.apiKey,
	}
}

func (vas *ValAPIService) doRequest(method, urlPath string, query map[string]string, data []byte) (body []byte, err error) {
	baseURL, err := url.Parse(vas.baseURL)
	if err != nil {
		return
	}

	log.Println(baseURL)

	apiURL, err := url.Parse(urlPath)
	if err != nil {
		return
	}

	endpoint := baseURL.ResolveReference(apiURL).String()
	log.Printf("action=doRequest endpoint=%s", endpoint)

	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	q := req.URL.Query()
	for key, value := range query {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	for key, value := range vas.getRequestHeader() {
		req.Header.Add(key, value)
	}

	resp, err := vas.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	newBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return newBody, nil
}

// GetPuuid 名前とタグからpuuidを取得する
func (vas *ValAPIService) GetPuuid(tagLine, name string) (string, error) {
	pathURL := fmt.Sprintf("/riot/account/v1/accounts/by-riot-id/%s/%s", name, tagLine)
	res, err := vas.doRequest("GET", pathURL, map[string]string{}, []byte{})
	log.Println(string(res))

	acount := &account{}
	json.Unmarshal(res, acount)
	return acount.Puuid, err
}
