package ai

import (
	"context"
	"encoding/json"
	"strings"
)

// Roles de un mensaje de la conversación con el modelo.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message es un mensaje de la conversación con el modelo.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Tool es una herramienta que el modelo puede pedir usar. Parameters es un esquema JSON.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall es una herramienta que el modelo pidió usar. Arguments es el JSON tal como llegó.
type ToolCall struct {
	Name      string
	Arguments string
}

// Argument devuelve el valor de texto de un argumento, o "" si falta o no es texto.
func (c ToolCall) Argument(name string) string {
	var args map[string]any
	if json.Unmarshal([]byte(c.Arguments), &args) != nil {
		return ""
	}
	v, _ := args[name].(string)
	return strings.TrimSpace(v)
}

// ChatRequest es una petición al modelo. MaxTokens acota la longitud de la respuesta.
type ChatRequest struct {
	Messages  []Message
	Tools     []Tool
	MaxTokens int
}

// ChatResponse es lo que contestó el modelo: texto, herramientas que pidió usar, o ambas.
type ChatResponse struct {
	Content          string
	ToolCalls        []ToolCall
	PromptTokens     int
	CompletionTokens int
}

// Chat le pide una respuesta al modelo. Los fallos son *Error.
func (c *Client) Chat(ctx context.Context, cfg Config, req ChatRequest) (ChatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()

	body := map[string]any{"model": cfg.Model, "messages": req.Messages}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": t.Name, "description": t.Description, "parameters": t.Parameters,
				},
			})
		}
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	data, err := c.post(ctx, cfg, body)
	if err != nil {
		return ChatResponse{}, err
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content   *string `json:"content"` // null cuando solo pide herramientas
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &out) != nil || len(out.Choices) == 0 {
		return ChatResponse{}, errNotOpenAI
	}

	msg := out.Choices[0].Message
	resp := ChatResponse{
		PromptTokens:     out.Usage.PromptTokens,
		CompletionTokens: out.Usage.CompletionTokens,
	}
	if msg.Content != nil {
		resp.Content = strings.TrimSpace(*msg.Content)
	}
	for _, tc := range msg.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	if resp.Content == "" && len(resp.ToolCalls) == 0 {
		return ChatResponse{}, fail(KindBadResponse, "el modelo respondió vacío")
	}
	return resp, nil
}
