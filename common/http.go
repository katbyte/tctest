package common

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

var Http = &http.Client{
	Timeout: time.Second * 10,
}

func HttpGetReader(url string) (*io.ReadCloser, error) {
	Log.Debug("HTTP GET: " + url)
	resp, err := Http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status NOT OK: %d", resp.StatusCode)
	}

	return &resp.Body, nil
}

func HttpReadByte(url string) (*[]byte, error) {
	r, err := HttpGetReader(url)
	if r != nil {
		defer (*r).Close()
	}
	if err != nil {
		return nil, fmt.Errorf("unable to get reader: %v", err)
	}

	body, err := ioutil.ReadAll(*r)
	if err != nil {
		return nil, fmt.Errorf("IO ReadAll error: %v", err)
	}

	return &body, nil
}

func HttpUnmarshalJson(url string, i interface{}) error {
	body, err := HttpReadByte(url)
	if err != nil {
		return fmt.Errorf("HTTP Get error: %v", err)
	}

	if err := json.Unmarshal(*body, &i); err != nil {
		return fmt.Errorf("JSON Unmarshal error: %v", err)
	}

	return nil
}
