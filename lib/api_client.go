package purobot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type APIClient struct {
	Prefix string
}

type Place struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Character struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Greeting struct {
	StartAt    string      `json:"start_at"`
	EndAt      string      `json:"end_at"`
	Deleted    bool        `json:"deleted"`
	Place      Place       `json:"place"`
	Characters []Character `json:"characters"`
}

const DefaultPrefix = "https://greeting.sucretown.net/api"

var (
	InternalError  = errors.New("internal error")
	NotFoundError  = errors.New("not found")
	TemporaryError = errors.New("temporary error")
)

func NewAPIClient() *APIClient {
	return &APIClient{
		Prefix: DefaultPrefix,
	}
}

func (c *APIClient) GetSchedule(date time.Time) ([]Greeting, error) {
	url := fmt.Sprintf("%s/schedule/%04d/%02d/%02d/", c.Prefix, date.Year(), date.Month(), date.Day())

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, NotFoundError
	}
	if res.StatusCode >= 500 && res.StatusCode <= 599 {
		return nil, TemporaryError
	}
	if res.StatusCode != 200 {
		return nil, InternalError
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var greetings []Greeting
	if err := json.Unmarshal(bytes, &greetings); err != nil {
		return nil, err
	}
	return greetings, nil
}
