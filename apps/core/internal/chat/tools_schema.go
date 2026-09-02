package chat

import "github.com/openai/openai-go/v2"

// toolSchemaParams returns the function definitions for the Chat Completions API.
func toolSchemaParams() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "discover_devices",
			Description: openai.String("Find appliances/devices accessible to the user that match a natural language description. Returns a list of matching appliances with their current state."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Natural language description of the device(s) to find, e.g. 'kitchen light' or 'bedroom fan'.",
					},
				},
				"required": []string{"query"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "check_device_online",
			Description: openai.String("Check whether the physical device backing this appliance is reachable. Always call this before control_device."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"appliance_id": map[string]any{
						"type":        "string",
						"description": "The UUID of the appliance to check (from discover_devices).",
					},
				},
				"required": []string{"appliance_id"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "get_device_state",
			Description: openai.String("Return the appliance's current state, e.g. {isOn: true}."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"appliance_id": map[string]any{
						"type":        "string",
						"description": "The UUID of the appliance to inspect (from discover_devices).",
					},
				},
				"required": []string{"appliance_id"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "control_device",
			Description: openai.String("Turn an appliance on or off and wait for the firmware acknowledgement. Returns {ok, message}. When ok is false, the command did NOT take effect - report the message faithfully and do not claim success."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"appliance_id": map[string]any{
						"type":        "string",
						"description": "The UUID of the appliance to control (from discover_devices).",
					},
					"action": map[string]any{
						"type":        "string",
						"enum":        []string{"on", "off"},
						"description": "The action to perform.",
					},
				},
				"required": []string{"appliance_id", "action"},
			},
		}),
	}
}
