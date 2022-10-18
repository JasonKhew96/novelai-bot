package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GenImageResp struct {
	Ptr   int64  `json:"ptr"`
	Image string `json:"image"`
	Final bool   `json:"final"`
	Error string `json:"error"`
}

type GenImageErrResp struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

type GenImageReqParameters struct {
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Scale         int    `json:"scale"`
	Sampler       string `json:"sampler"`
	Steps         int    `json:"steps"`
	Seed          int64  `json:"seed"`
	NSamples      int    `json:"n_samples"`
	UCPreset      int    `json:"ucPreset"`
	QualityToggle bool   `json:"qualityToggle"`
	UC            string `json:"uc"`
}

type GenImageReq struct {
	Input      string                `json:"input"`
	Model      string                `json:"model"`
	Parameters GenImageReqParameters `json:"parameters"`
}

func genImage(bearerToken, input string, isNSFW, isLandscape bool) ([]byte, error) {
	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	width := 512
	height := 768
	if isLandscape {
		width = 768
		height = 512
	}

	if !strings.Contains(input, "masterpiece") {
		input = "masterpiece, " + input
	}
	if !strings.Contains(input, "best quality") {
		input = "best quality, " + input
	}

	undesiredContent := "lowres, bad anatomy, bad hands, text, error, missing fingers, extra digit, fewer digits, cropped, worst quality, low quality, normal quality, jpeg artifacts, signature, watermark, username, blurry"
	if isNSFW {
		if !strings.Contains(input, "nsfw") {
			input = "nsfw, " + input
		}
		undesiredContent = "nsfw, " + undesiredContent
	}

	reqJson := GenImageReq{
		Input: input,
		Model: "nai-diffusion",
		Parameters: GenImageReqParameters{
			Width:         width,
			Height:        height,
			Scale:         11,
			Sampler:       "k_euler_ancestral",
			Steps:         28,
			Seed:          0,
			NSamples:      1,
			UCPreset:      0,
			QualityToggle: true,
			UC:            undesiredContent,
		},
	}

	data, err := json.Marshal(&reqJson)
	if err != nil {
		return nil, err
	}

	reqBody := bytes.NewBuffer(data)

	// reqUrl := "https://api.novelai.net/ai/generate-image"
	reqUrl := "https://backend-production-svc.novelai.net/ai/generate-image"
	req, err := http.NewRequest(http.MethodPost, reqUrl, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var respJson GenImageErrResp
		if err := json.Unmarshal(body, &respJson); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("status code: %d, message: %s", respJson.StatusCode, respJson.Message)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	newBody := string(body)
	splits := strings.Split(newBody, "\n")

	if len(splits) < 3 {
		return nil, fmt.Errorf("wrong data len: %d", len(splits))
	}

	if splits[0] != "event: newImage" {
		return nil, fmt.Errorf("wrong event: %s", newBody)
	}

	if !strings.HasPrefix(splits[2], "data:") {
		return nil, fmt.Errorf("data not found: %s", newBody)
	}

	imageData := []byte(strings.TrimPrefix(splits[2], "data:"))

	return imageData, nil
}
