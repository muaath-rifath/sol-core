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
			"name":        "check_device_online",
			"description": "Check whether the physical device backing this appliance is reachable. Always call this before control_device.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"appliance_id": map[string]any{
						"type":        "string",
						"description": "The UUID of the appliance to check (from discover_devices).",
					},
				},
				"required": []string{"appliance_id"},
			},
		},
		{
			"type":        "function",
			"name":        "get_device_state",
			"description": "Return the appliance's current state, e.g. {isOn: true}.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"appliance_id": map[string]any{
						"type":        "string",
						"description": "The UUID of the appliance to inspect (from discover_devices).",
					},
				},
				"required": []string{"appliance_id"},
			},
		},
		{
			"type":        "function",
			"name":        "control_device",
			"description": "Turn an appliance on or off and wait for the firmware acknowledgement. Returns {ok, message}. When ok is false, the command did NOT take effect - report the message faithfully and do not claim success.",
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
