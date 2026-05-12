package chat

// toolSchemas returns the function definitions for the Azure OpenAI Realtime session.update event.
func toolSchemas() []map[string]any {
	return []map[string]any{
		{
			"type": "function",
			"name": "discover_devices",
			"description": "Find appliances/devices accessible to the user that match a natural language description. " +
				"Returns a list of matching appliances with their current state.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Natural language description of the device(s) to find, e.g. 'kitchen light' or 'bedroom fan'.",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"type":        "function",
			"name":        "control_device",
			"description": "Turn an appliance on or off. Only works for appliances the user has permission to control.",
			"parameters": map[string]any{
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
		},
	}
}
