package openaillm

import (
	"context"
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/llms/prompts"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"k8s.io/klog/v2"
)

type PodCondition struct {
	Running bool `json:"running"`
}

type PodAdvice struct {
	Solution string `json:"solution"`
}

func New() *openai.Client {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	return client
}

func CheckPodHeathy(client *openai.Client, content string) (bool, error) {
	podCondition := PodCondition{}
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: prompts.CheckPodHeathy(content),
			Functions: []openai.FunctionDefinition{{
				Name: "get_status",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"running": {
							Type:        jsonschema.Boolean,
							Description: "Pod is running healthy without any error",
						},
					},
				},
			}},
		},
	)

	if err != nil {
		klog.Error("ChatCompletion error: %v\n", err)
		return false, err
	}

	err = json.Unmarshal([]byte(resp.Choices[0].Message.FunctionCall.Arguments), &podCondition)

	if err != nil {
		klog.Error("Unmarshal error: %v\n", err)
		return false, err
	}

	return podCondition.Running, nil
}

func GetAdviceFromUnhealthyPod(client *openai.Client, content string) (string, error) {
	advice := PodAdvice{}
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleSystem,
					Content: `You are a kubernetes expert that solve any k8s issue. You will be asked for explaining the error message from a unhealthy pod. Please provide a short explanation and the most possible solution in a step by step style in no more than 280 characters. Write the output in the json format:
					{
						"error": "short explanation of error message here",
						"solution": "solution here"
					}`,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: `The following are the details of the pod: ### ` + content + ` ###`,
				},
			},
		},
	)
	if err != nil {
		klog.Error("ChatCompletion error: %v\n", err)
		return "", errors.Wrap(err, "failed to finish openai chatCompletion")
	}

	json.Unmarshal([]byte(resp.Choices[0].Message.Content), &advice)
	return advice.Solution, nil
}
