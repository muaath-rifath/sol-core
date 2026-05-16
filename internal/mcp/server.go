package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/muaathrifath/sol-core/internal/device"
	"github.com/muaathrifath/sol-core/internal/permission"
	"github.com/muaathrifath/sol-core/internal/room"
	"github.com/muaathrifath/sol-core/internal/user"
)

type Server struct {
	mcpServer *mcp.Server
	deviceSvc *device.Service
	roomSvc   *room.Service
	permSvc   *permission.Service
}

func NewServer(deviceSvc *device.Service, roomSvc *room.Service, permSvc *permission.Service) *Server {
	s := &Server{
		deviceSvc: deviceSvc,
		roomSvc:   roomSvc,
		permSvc:   permSvc,
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
	type ListAppliancesArgs struct {
		HomeID string `json:"home_id"`
	}

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_appliances",
		Description: "List appliances in the smart home that the caller has access to and their current power states. Requires home_id.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListAppliancesArgs) (*mcp.CallToolResult, any, error) {
		u := user.FromContext(ctx)
		if u == nil {
			return nil, nil, fmt.Errorf("unauthorized")
		}
		if args.HomeID == "" {
			return nil, nil, fmt.Errorf("home_id is required")
		}

		accessibleIDs, allAccess, err := s.permSvc.ListAccessibleApplianceIDs(ctx, args.HomeID, u.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to check permissions: %w", err)
		}

		apps, err := s.deviceSvc.ListAllAppliances(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list appliances: %w", err)
		}

		// Build lookup set for non-owner access
		allowed := make(map[string]bool, len(accessibleIDs))
		for _, id := range accessibleIDs {
			allowed[id] = true
		}

		var sb strings.Builder
		sb.WriteString("Available Appliances:\n")
		for _, app := range apps {
			if !allAccess && !allowed[app.ID] {
				continue
			}
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
		u := user.FromContext(ctx)
		if u == nil {
			return nil, nil, fmt.Errorf("unauthorized")
		}

		ok, err := s.permSvc.CheckAppliance(ctx, u.ID, args.ApplianceID)
		if err != nil {
			return nil, nil, fmt.Errorf("permission check failed: %w", err)
		}
		if !ok {
			return nil, nil, fmt.Errorf("access denied to appliance %s", args.ApplianceID)
		}

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
