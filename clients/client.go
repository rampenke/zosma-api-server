package main

import (
     "os"
     "encoding/json"
     "encoding/base64"
     "image"
     "image/png"
     "strings"
    "context"
    "fmt"
    "log"
    "time"

    "github.com/hibiken/asynq"
    "github.com/rampenke/zosma-api-server/tasks"
)

const redisAddr = "127.0.0.1:6379"

func waitForResult(ctx context.Context, i *asynq.Inspector, queue, taskID string) (*asynq.TaskInfo, error) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			taskInfo, err := i.GetTaskInfo(queue, taskID)
			if err != nil {
				return nil, err
			}
			if taskInfo.CompletedAt.IsZero() {
				continue
			}
			return taskInfo, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("context closed")
		}
	}
}

func main() {
    conn := asynq.RedisClientOpt{Addr: redisAddr}
    client := asynq.NewClient(conn)
    defer client.Close()
    inspector := asynq.NewInspector(conn)
    request := &tasks.TextToImageRequest{
	    Prompt : "Blue Ocean",
	    NegativePrompt: "ugly, tiling, poorly drawn hands, poorly drawn feet, poorly drawn face, out of frame, " +
				"mutation, mutated, extra limbs, extra legs, extra arms, disfigured, deformed, cross-eye, " +
				"body out of frame, blurry, bad art, bad anatomy, blurred, text, watermark, grainy",
	    Width: 512,
	    Height: 512,
	    RestoreFaces: true,
	    EnableHR:	false,
	    HRResizeX: 512,
	    HRResizeY: 512,
	    DenoisingStrength:  0.7,
	    BatchSize: 1,
	    Seed: -1,
	    Subseed: -1,
	    SubseedStrength: 0,
	    SamplerName: "Euler a",
	    CfgScale: 9,
	    Steps: 20,
	    NIter: 1,
    }
    task, err := tasks.NewTxt2imgTask(request)
    if err != nil {
        log.Fatalf("could not create task: %v", err)
    }
    info, err := client.Enqueue(task, asynq.MaxRetry(10), asynq.Timeout(3 * time.Minute), asynq.Retention(2 * time.Hour))
    if err != nil {
        log.Fatalf("could not enqueue task: %v", err)
    }
    log.Printf("enqueued task: id=%s queue=%s", info.ID, info.Queue)
   ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
   defer cancel()
   res, err := waitForResult(ctx, inspector, "default", info.ID)
   if err != nil {
        log.Fatalf("unable to wait for resilt: %v", err)
   }
   var respStruct = &tasks.TextToImageResponse{};
   err = json.Unmarshal(res.Result, respStruct)
   if err != nil {
	log.Fatalf("Unexpected API response: %v", err)
   }
   //fmt.Printf("result: %v", respStruct)
   reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(string(respStruct.Images[0])))
   
   f, err := os.OpenFile("sample1.png", os.O_WRONLY|os.O_CREATE, 0777)
   if err != nil {
        log.Fatal(err)
        return
   }
   m, _, err := image.Decode(reader)
   if err != nil {
	log.Fatalf("image.Decode error: %v", err)
   }
  err = png.Encode(f, m)
  if err != nil {
	log.Fatalf("png.Encode error: %v", err)
        return
  }
}
