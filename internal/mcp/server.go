// TODO(permissions): wire permission.Service into MCP. Steps:
//  1. Wrap mcp/sse mount in cmd/sol/main.go with authMiddleware so tool calls
//     have a *user.User on the context.
//  2. Pass *permission.Service into NewServer.
//  3. list_appliances: require home_id arg; call
//     permSvc.ListAccessibleApplianceIDs(ctx, homeID, u.ID); filter
//     deviceSvc.ListAllAppliances by the returned IDs (skip filter when
//     allAccess is true).
//  4. set_appliance_state: call permSvc.CheckAppliance(ctx, u.ID,
//     args.ApplianceID) before issuing the command.

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/muaathrifath/sol-core/internal/device"
	"github.com/muaathrifath/sol-core/internal/room"
)

type Server struct {
	mcpServer *mcp.Server
	deviceSvc *device.Service
	roomSvc   *room.Service
}

func NewServer(deviceSvc *device.Service, roomSvc *room.Service) *Server {
	s := &Server{
		deviceSvc: deviceSvc,
		roomSvc:   roomSvc,
	}

	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "sol-home-automation",
		Version: "1.0.0",
	}, nil)

	s.registerTools()

	return s
}

func (s *Server) registerTools() {
	// Tool: list_appliances
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_appliances",
		Description: "List all available appliances in the smart home and their current power states.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		apps, err := s.deviceSvc.ListAllAppliances(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list appliances: %w", err)
		}

		var sb strings.Builder
		sb.WriteString("Available Appliances:\n")
		for _, app := range apps {
			isOn := false
			if app.State != nil {
				if val, ok := app.State["isOn"].(bool); ok {
					isOn = val
				}
			}
			stateStr := "OFF"
			if isOn {
				stateStr = "ON"
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (%s) is %s\n", app.ID, app.Name, app.Type, stateStr))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: sb.String(),
				},
			},
		}, nil, nil
	})

	// Tool: set_appliance_state
	type SetApplianceStateArgs struct {
		ApplianceID string `json:"appliance_id"`
		State       bool   `json:"state"`
	}

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "set_appliance_state",
		Description: "Control the power state of a specific appliance by its ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SetApplianceStateArgs) (*mcp.CallToolResult, any, error) {
		app, err := s.deviceSvc.GetAppliance(ctx, args.ApplianceID)
		if err != nil {
			return nil, nil, fmt.Errorf("appliance not found: %w", err)
		}

		params := map[string]any{
			"power": args.State,
		}
		if app.Channel != nil {
			params["channel"] = *app.Channel
		}

		cmd := device.Command{
			DeviceID: app.DeviceID,
			Action:   "set",
			Params:   params,
		}

		if err := s.deviceSvc.SendCommand(ctx, cmd); err != nil {
			return nil, nil, fmt.Errorf("failed to send command to device: %w", err)
		}

		stateStr := "OFF"
		if args.State {
			stateStr = "ON"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Successfully turned %s %s.", app.Name, stateStr),
				},
			},
		}, nil, nil
	})
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)
}
