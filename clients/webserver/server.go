package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rampenke/zosma-sd-server/tasks"
)

type Config struct {
	RedisAddr     string `envconfig:"REDIS_ADDR" required:"true"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" required:"true"`
}

var cfg Config

type Server struct {
	e         *echo.Echo
	client    *asynq.Client
	inspector *asynq.Inspector
}

const (
	MaxPending = 1
)

var (
	ErrLongWaitQueue = errors.New("Long wait queue")
)

type HttpError struct {
	Error        string `json:"error"`
	ErrorMessage string `json:"error_description"`
}

func NewBadRequest(c echo.Context) error {
	status := http.StatusBadRequest
	return NewHttpError(c, status, errors.New(http.StatusText(status)))
}

func NewLongWaitQueueError(c echo.Context) error {
	status := http.StatusServiceUnavailable
	return NewHttpError(c, status, ErrLongWaitQueue)
}

func NewHttpError(c echo.Context, status int, err error) error {
	errStr := err.Error()
	errCode := strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_")
	errMessage := ""
	if err != nil {
		errMessage = strings.ToTitle(string(errStr[0])) + errStr[1:]
	} else {
		errMessage = http.StatusText(status)
	}
	return echo.NewHTTPError(status, &HttpError{
		Error:        errCode,
		ErrorMessage: errMessage,
	})
}

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

// func (s *Server)ProcessQuery(c echo.Context, client *asynq.Client, inspector *asynq.Inspector) (*tasks.TextToImageResponse, error) {
func (s *Server) ProcessQuery(c echo.Context) error {

	request := &tasks.TextToImageRequest{}
	err := c.Bind(&request)
	if err != nil {
		return NewBadRequest(c)
	}

	task, err := tasks.NewTxt2imgTask(request)
	if err != nil {
		log.Printf("could not create task: %v", err)
		return NewBadRequest(c)
	}
	/*
		qInfo, err := s.inspector.GetQueueInfo(tasks.Txt2imgQueue)
		if err != nil {
			log.Printf("could not get queue info: %v", err)
			return NewBadRequest(c)
		}

		if qInfo.Pending >= MaxPending {
			return NewLongWaitQueueError(c)
		}
	*/
	info, err := s.client.Enqueue(task, asynq.Queue(tasks.Txt2imgQueue), asynq.MaxRetry(10), asynq.Timeout(3*time.Minute), asynq.Retention(2*time.Hour))
	if err != nil {
		log.Printf("could not enqueue task: %v", err)
		return NewBadRequest(c)
	}
	log.Printf("enqueued task: id=%s queue=%s", info.ID, info.Queue)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res, err := waitForResult(ctx, s.inspector, tasks.Txt2imgQueue, info.ID)
	if err != nil {
		log.Printf("unable to wait for resilt: %v", err)
		return NewBadRequest(c)
	}

	var respStruct = &tasks.TextToImageResponse{}
	err = json.Unmarshal(res.Result, respStruct)
	if err != nil {
		log.Printf("Unexpected API response: %v", err)
		return c.JSON(http.StatusBadRequest, err)
	}

	response := &tasks.TextToImageResponse{
		Images:   respStruct.Images,
		Seeds:    respStruct.Seeds,
		Subseeds: respStruct.Subseeds,
	}

	return c.JSON(http.StatusOK, response)
}

func (s *Server) Start(errCh chan error) {
	_ = godotenv.Overload()
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatal(err.Error())
	}
	conn := asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword}
	s.client = asynq.NewClient(conn)
	// defer client.Close()
	s.inspector = asynq.NewInspector(conn)

	s.e = echo.New()
	s.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*", "https://zosma.mcntech.com"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	s.e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	s.e.POST("/sdapi/v1/txt2img", s.ProcessQuery)

	go func() {
		err := s.e.Start(":1324")
		errCh <- err
	}()
}

func (s *Server) Stop() error {
	return s.e.Shutdown(context.Background())
}

func main() {

	signals := make(chan os.Signal, 1)
	exit := make(chan bool, 1)
	errCh := make(chan error, 1)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signals
		log.Printf("Received signal %s", sig.String())
		exit <- true
	}()
	s := &Server{}
	go s.Start(errCh)

	select {
	case <-exit:
		break
	case err := <-errCh:
		if err != nil {
			log.Printf("%v", err)
		}
	}
	log.Print("Stopping...")
	_ = s.Stop()
	log.Print("Stoppped")
}
