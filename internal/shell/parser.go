package shell

import (
	"fmt"
	"strings"

	"fix-tool/internal/order"
)

type CommandKind string

const (
	CommandLogon        CommandKind = "logon"
	CommandLogout       CommandKind = "logout"
	CommandHeartbeat    CommandKind = "heartbeat"
	CommandTestRequest  CommandKind = "test-request"
	CommandOrderNew     CommandKind = "order new"
	CommandOrderCancel  CommandKind = "order cancel"
	CommandOrderReplace CommandKind = "order replace"
	CommandTraceList    CommandKind = "trace list"
	CommandExit         CommandKind = "exit"
)

type Command struct {
	Kind           CommandKind
	TestRequestID  string
	NewRequest     order.NewRequest
	CancelRequest  order.CancelRequest
	ReplaceRequest order.ReplaceRequest
}

func Parse(line string) (Command, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}
	switch fields[0] {
	case "logon":
		return noArgCommand(fields, Command{Kind: CommandLogon})
	case "logout":
		return noArgCommand(fields, Command{Kind: CommandLogout})
	case "heartbeat":
		return noArgCommand(fields, Command{Kind: CommandHeartbeat})
	case "exit":
		return noArgCommand(fields, Command{Kind: CommandExit})
	case "test-request":
		return parseTestRequest(fields[1:])
	case "trace":
		return parseTrace(fields[1:])
	case "order":
		return parseOrder(fields[1:])
	default:
		return Command{}, fmt.Errorf("unknown command %q", fields[0])
	}
}

func noArgCommand(fields []string, command Command) (Command, error) {
	if len(fields) != 1 {
		return Command{}, fmt.Errorf("%s does not accept arguments", fields[0])
	}
	return command, nil
}

func parseTestRequest(args []string) (Command, error) {
	values, err := parseFlags(args, map[string]bool{"id": true})
	if err != nil {
		return Command{}, err
	}
	return Command{
		Kind:          CommandTestRequest,
		TestRequestID: singleValue(values, "id"),
	}, nil
}

func parseTrace(args []string) (Command, error) {
	if len(args) == 1 && args[0] == "list" {
		return Command{Kind: CommandTraceList}, nil
	}
	return Command{}, fmt.Errorf("trace supports only: trace list")
}

func parseOrder(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{}, fmt.Errorf("order requires subcommand")
	}
	switch args[0] {
	case "new":
		return parseOrderNew(args[1:])
	case "cancel":
		return parseOrderCancel(args[1:])
	case "replace":
		return parseOrderReplace(args[1:])
	default:
		return Command{}, fmt.Errorf("unknown order subcommand %q", args[0])
	}
}

func parseOrderNew(args []string) (Command, error) {
	values, err := parseFlags(args, map[string]bool{
		"cl-ord-id":     true,
		"symbol":        true,
		"side":          true,
		"qty":           true,
		"price":         true,
		"ord-type":      true,
		"time-in-force": true,
		"tag":           true,
	})
	if err != nil {
		return Command{}, err
	}
	return Command{
		Kind: CommandOrderNew,
		NewRequest: order.NewRequest{
			ClOrdID:     singleValue(values, "cl-ord-id"),
			Symbol:      singleValue(values, "symbol"),
			Side:        singleValue(values, "side"),
			OrderQty:    singleValue(values, "qty"),
			Price:       singleValue(values, "price"),
			OrdType:     singleValue(values, "ord-type"),
			TimeInForce: singleValue(values, "time-in-force"),
			Tags:        values["tag"],
		},
	}, nil
}

func parseOrderCancel(args []string) (Command, error) {
	values, err := parseFlags(args, map[string]bool{
		"orig-cl-ord-id": true,
		"cl-ord-id":      true,
		"order-id":       true,
		"symbol":         true,
		"side":           true,
		"tag":            true,
	})
	if err != nil {
		return Command{}, err
	}
	return Command{
		Kind: CommandOrderCancel,
		CancelRequest: order.CancelRequest{
			OrigClOrdID: singleValue(values, "orig-cl-ord-id"),
			ClOrdID:     singleValue(values, "cl-ord-id"),
			OrderID:     singleValue(values, "order-id"),
			Symbol:      singleValue(values, "symbol"),
			Side:        singleValue(values, "side"),
			Tags:        values["tag"],
		},
	}, nil
}

func parseOrderReplace(args []string) (Command, error) {
	values, err := parseFlags(args, map[string]bool{
		"orig-cl-ord-id": true,
		"cl-ord-id":      true,
		"order-id":       true,
		"symbol":         true,
		"side":           true,
		"qty":            true,
		"price":          true,
		"ord-type":       true,
		"time-in-force":  true,
		"tag":            true,
	})
	if err != nil {
		return Command{}, err
	}
	return Command{
		Kind: CommandOrderReplace,
		ReplaceRequest: order.ReplaceRequest{
			OrigClOrdID: singleValue(values, "orig-cl-ord-id"),
			ClOrdID:     singleValue(values, "cl-ord-id"),
			OrderID:     singleValue(values, "order-id"),
			Symbol:      singleValue(values, "symbol"),
			Side:        singleValue(values, "side"),
			OrderQty:    singleValue(values, "qty"),
			Price:       singleValue(values, "price"),
			OrdType:     singleValue(values, "ord-type"),
			TimeInForce: singleValue(values, "time-in-force"),
			Tags:        values["tag"],
		},
	}, nil
}

func parseFlags(args []string, allowed map[string]bool) (map[string][]string, error) {
	values := map[string][]string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			return nil, fmt.Errorf("unexpected argument %q", arg)
		}
		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if name == "" {
			return nil, fmt.Errorf("empty flag name")
		}
		if !allowed[name] {
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
		if !hasValue {
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "--") {
				return nil, fmt.Errorf("missing value for --%s", name)
			}
			value = args[i]
		}
		values[name] = append(values[name], value)
	}
	return values, nil
}

func singleValue(values map[string][]string, name string) string {
	items := values[name]
	if len(items) == 0 {
		return ""
	}
	return items[len(items)-1]
}
