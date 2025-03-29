package dify

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type ChatMessageStreamResponse struct {
	Event          string `json:"event"`
	TaskID         string `json:"task_id"`
	ID             string `json:"id"`
	Answer         string `json:"answer"`
	CreatedAt      int64  `json:"created_at"`
	ConversationID string `json:"conversation_id"`
	Type           string `json:"type"`
	BelongsTo      string `json:"belongs_to"`
	Url            string `json:"url"`
	Audio          string `json:"audio"`
	WorkflowRunId  string `json:"workflow_run_id"`
	Metadata       struct {
		Usage              interface{}   `json:"usage"`
		RetrieverResources []interface{} `json:"retriever_resources"`
	} `json:"metadata"`
	Data struct {
		Id                string      `json:"id"`
		CreatedAt         int64       `json:"created_at"`
		FinishedAt        int64       `json:"finished_at"`
		WorkflowId        string      `json:"workflow_id"`
		SequenceNumber    int         `json:"sequence_number"`
		NodeId            string      `json:"node_id"`
		NodeType          string      `json:"node_type"`
		Title             string      `json:"title"`
		Index             int         `json:"index"`
		PredecessorNodeId string      `json:"predecessor_node_id"`
		ProcessData       string      `json:"process_data"`
		Status            string      `json:"status"`
		Error             string      `json:"error"`
		ElapsedTime       float32     `json:"elapsed_time"`
		Inputs            interface{} `json:"inputs,omitempty"`
		Outputs           interface{} `json:"outputs"`
		TotalTokens       int         `json:"total_tokens"`
		TotalSteps        int         `json:"total_steps"`
		ExecutionMetadata struct {
			TotalTokens int     `json:"total_tokens"`
			TotalPrice  float64 `json:"total_price,omitempty"`
			Currency    string  `json:"currency,omitempty"`
		} `json:"execution_metadata"`
	} `json:"data"`
}

type ChatMessageStreamChannelResponse struct {
	ChatMessageStreamResponse
	Err error `json:"-"`
}

func (api *API) ChatMessagesStreamRaw(ctx context.Context, req *ChatMessageRequest) (*http.Response, error) {
	req.ResponseMode = "streaming"

	httpReq, err := api.createBaseRequest(ctx, http.MethodPost, "/v1/chat-messages", req)
	if err != nil {
		return nil, err
	}
	return api.c.sendRequest(httpReq)
}

func (api *API) ChatMessagesStream(ctx context.Context, req *ChatMessageRequest) (chan ChatMessageStreamChannelResponse, error) {
	httpResp, err := api.ChatMessagesStreamRaw(ctx, req)
	if err != nil {
		return nil, err
	}

	streamChannel := make(chan ChatMessageStreamChannelResponse)
	go api.chatMessagesStreamHandle(ctx, httpResp, streamChannel)
	return streamChannel, nil
}

func (api *API) chatMessagesStreamHandle(ctx context.Context, resp *http.Response, streamChannel chan ChatMessageStreamChannelResponse) {
	defer resp.Body.Close()
	defer close(streamChannel)

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				streamChannel <- ChatMessageStreamChannelResponse{
					Err: fmt.Errorf("error reading line: %w", err),
				}
				return
			}

			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			line = bytes.TrimPrefix(line, []byte("data:"))

			var resp ChatMessageStreamChannelResponse
			if err = json.Unmarshal(line, &resp); err != nil {
				streamChannel <- ChatMessageStreamChannelResponse{
					Err: fmt.Errorf("error unmarshalling event: %w", err),
				}
				return
			} else if resp.Event == "error" {
				streamChannel <- ChatMessageStreamChannelResponse{
					Err: errors.New("error streaming event: " + string(line)),
				}
				return
			}
			streamChannel <- resp
			if resp.Event == "workflow_finished" {
				return
			}
		}
	}
}
