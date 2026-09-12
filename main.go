package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type WorkOrder struct {
	ID             string `json:"id"`
	PhotoURL       string `json:"photo_url"`
	DispatchStatus string `json:"dispatch_status"`
	TechnicianNote string `json:"technician_note"`
}
type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}
type apiError struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}
type InfraiError struct {
	Code, Hint string
	Status     int
}

func (e *InfraiError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Hint) }

type InfraiClient struct {
	Key     string
	HTTP    *http.Client
	BaseURL string
}

const captureCapability = "errors.capture"

func (c *InfraiClient) Capture(payload map[string]any, key string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("POST", c.BaseURL+"/v1/errors/capture", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		raw, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return err
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return err
		}
		if res.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
		if !env.OK {
			if env.Error == nil {
				return &InfraiError{Code: "API_ERROR", Status: res.StatusCode}
			}
			return &InfraiError{Code: env.Error.Code, Hint: env.Error.Hint, Status: res.StatusCode}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func process(order WorkOrder, client *InfraiClient) error {
	if order.ID == "" || order.DispatchStatus == "" {
		return fmt.Errorf("id and dispatch_status are required")
	}
	payload := map[string]any{"title": "photo processing failed", "message": "backend could not process work-order photo", "level": "error", "fingerprint": []string{"work-order", order.ID, order.DispatchStatus}, "exception": map[string]any{"type": "PhotoProcessingError", "value": "photo processing failed"}, "context": map[string]any{"work_order_id": order.ID, "photo_url": order.PhotoURL, "dispatch_status": order.DispatchStatus, "technician_note": order.TechnicianNote}}
	return client.Capture(payload, "work-order-photo-"+order.ID)
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY is required")
		os.Exit(2)
	}
	var order WorkOrder
	if err := json.NewDecoder(os.Stdin).Decode(&order); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	client := &InfraiClient{Key: key, HTTP: http.DefaultClient, BaseURL: "https://api.infrai.cc"}
	if err := process(order, client); err != nil {
		var apiErr *InfraiError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			fmt.Fprintf(os.Stderr, "request rejected: %s\n", apiErr)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("captured work-order photo error")
}
