package prompts

import "github.com/sashabaranov/go-openai"

func CheckPodHeathy(content string) []openai.ChatCompletionMessage {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: `You are a kubernetes expert that detect any k8s issue. You will be asked for detecting the issue of the pod.`,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: `The following are the details of the pod: ### ` + content + ` ###`,
		},
	}

	return messages
}
