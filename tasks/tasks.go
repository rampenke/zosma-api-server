package tasks

import (
    "bytes"
    "context"
    "errors"
    "io"
    "encoding/json"
    "fmt"
    "log"
    "time"
    "github.com/hibiken/asynq"
    "net/http"
)

// A list of task types.
const (
    TypeTxt2img     = "text:image"
)

type JsonTextToImageResponse struct {
	Images []string `json:"images"`
	Info   string   `json:"info"`
}

type JsonInfoResponse struct {
	Seed        int   `json:"seed"`
	AllSeeds    []int `json:"all_seeds"`
	AllSubseeds []int `json:"all_subseeds"`
}

type TextToImageRequest struct {
	Prompt            string  `json:"prompt"`
	NegativePrompt    string  `json:"negative_prompt"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	RestoreFaces      bool    `json:"restore_faces"`
	EnableHR          bool    `json:"enable_hr"`
	HRResizeX         int     `json:"hr_resize_x"`
	HRResizeY         int     `json:"hr_resize_y"`
	DenoisingStrength float64 `json:"denoising_strength"`
	BatchSize         int     `json:"batch_size"`
	Seed              int     `json:"seed"`
	Subseed           int     `json:"subseed"`
	SubseedStrength   float64 `json:"subseed_strength"`
	SamplerName       string  `json:"sampler_name"`
	CfgScale          float64 `json:"cfg_scale"`
	Steps             int     `json:"steps"`
	NIter             int     `json:"n_iter"`
}

type TextToImageResponse struct {
	Images   []string `json:"images"`
	Seeds    []int    `json:"seeds"`
	Subseeds []int    `json:"subseeds"`
}

//----------------------------------------------
// Write a function NewXXXTask to create a task.
// A task consists of a type and a payload.
//----------------------------------------------


func NewTxt2imgTask(req *TextToImageRequest) (*asynq.Task, error) {
    payload, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }
    // task options can be passed to NewTask, which can be overridden at enqueue time.
    return asynq.NewTask(TypeTxt2img, payload, asynq.MaxRetry(5), asynq.Timeout(20 * time.Minute)), nil
}

//---------------------------------------------------------------
// Write a function HandleXXXTask to handle the input task.
// Note that it satisfies the asynq.HandlerFunc interface.
//
// Handler doesn't need to be a function. You can define a type
// that satisfies asynq.Handler interface. See examples below.
//---------------------------------------------------------------


// ImageProcessor implements asynq.Handler interface.
type Txt2imgProcessor struct {
    // ... fields for struct
    host string
}

func (api *Txt2imgProcessor) TextToImage(req *TextToImageRequest) (*TextToImageResponse, error) {
	if req == nil {
		return nil, errors.New("missing request")
	}

	postURL := api.host + "/sdapi/v1/txt2img"

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", postURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("API URL: %s", postURL)
		log.Printf("Error with API Request: %s", string(jsonData))

		return nil, err
	}

	defer response.Body.Close()

	body, _ := io.ReadAll(response.Body)

	respStruct := &JsonTextToImageResponse{}

	err = json.Unmarshal(body, respStruct)
	if err != nil {
		log.Printf("API URL: %s", postURL)
		log.Printf("Unexpected API response: %s", string(body))

		return nil, err
	}

	infoStruct := &JsonInfoResponse{}

	err = json.Unmarshal([]byte(respStruct.Info), infoStruct)
	if err != nil {
		log.Printf("API URL: %s", postURL)
		log.Printf("Unexpected API response: %s", string(body))

		return nil, err
	}

	return &TextToImageResponse{
		Images:   respStruct.Images,
		Seeds:    infoStruct.AllSeeds,
		Subseeds: infoStruct.AllSubseeds,
	}, nil
}

func (processor *Txt2imgProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
    var p TextToImageRequest
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
    }
    log.Printf("Generating image for prompt: %s", p.Prompt)
    //res := []byte("Welcome")
    res, err := processor.TextToImage(&p)
    if err != nil {
        log.Printf("TextToImage Failed: %v", err)
        return err
    }
    jsonRes, err := json.Marshal(res)
    if err != nil {
            return err
    }

    if _, err := t.ResultWriter().Write(jsonRes); err != nil {
        log.Printf("Failed to write task result: %v", err)
        return err
    }
    // Image resizing code ...
    return nil
}

func NewTxt2imgProcessor() *Txt2imgProcessor {
	return &Txt2imgProcessor{ host: "http://127.0.0.1:7860"}
}
