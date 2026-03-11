// ClawFactory CLI tool entry point.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	baseURL    string
	apiToken   string
	outputJSON bool
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
)

// statusColors maps status strings to ANSI color codes.
var statusColors = map[string]string{
	"online":       colorGreen,
	"completed":    colorGreen,
	"offline":      colorRed,
	"failed":       colorRed,
	"deregistered": colorYellow,
	"running":      colorYellow,
	"assigned":     colorYellow,
	"pending":      colorYellow,
}

// colorize wraps a status string with the corresponding ANSI color code.
func colorize(status string) string {
	if color, ok := statusColors[status]; ok {
		return color + status + colorReset
	}
	return status
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "claw",
		Short: "ClawFactory CLI - multi-agent orchestration platform command-line tool",
	}

	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "http://localhost:8080", "ClawFactory server address")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "dev-token-001", "API Token")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "output-json", false, "output in JSON format")

	// workflow command group
	workflowCmd := &cobra.Command{Use: "workflow", Short: "Workflow management"}
	workflowCmd.AddCommand(workflowSubmitCmd(), workflowStatusCmd(), workflowArtifactsCmd(), workflowListCmd())

	// agent command group
	agentCmd := &cobra.Command{Use: "agent", Short: "Agent management"}
	agentCmd.AddCommand(agentListCmd(), agentLogsCmd(), agentDeregisterCmd())

	rootCmd.AddCommand(workflowCmd, agentCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to ClawFactory service (%s): %w", baseURL, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func workflowSubmitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submit [workflow.json]",
		Short: "Submit a workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			var def map[string]interface{}
			if err := json.Unmarshal(data, &def); err != nil {
				return fmt.Errorf("failed to parse JSON: %w", err)
			}
			resp, err := doRequest("POST", "/v1/admin/workflows", def)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var result map[string]interface{}
				json.Unmarshal(resp, &result)
				fmt.Printf("Workflow submitted: %v\n", result["instance_id"])
			}
			return nil
		},
	}
}

func workflowStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [workflow_id]",
		Short: "Query workflow status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/workflows/"+args[0], nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var result map[string]interface{}
				json.Unmarshal(resp, &result)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "ID\t%v\n", result["instance_id"])
				fmt.Fprintf(w, "Status\t%v\n", colorize(fmt.Sprintf("%v", result["status"])))
				w.Flush()
			}
			return nil
		},
	}
}

func workflowArtifactsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "artifacts [workflow_id]",
		Short: "Query workflow artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/workflows/"+args[0]+"/artifacts", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var artifacts []map[string]interface{}
				json.Unmarshal(resp, &artifacts)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "WORKFLOW\tTASK\tNAME\tPATH\n")
				for _, a := range artifacts {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", a["workflow_id"], a["task_id"], a["name"], a["path"])
				}
				w.Flush()
			}
			return nil
		},
	}
}

func agentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/agents", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var agents []map[string]interface{}
				json.Unmarshal(resp, &agents)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tNAME\tSTATUS\tVERSION\tCAPABILITIES\n")
				for _, a := range agents {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
						a["agent_id"], a["name"], colorize(fmt.Sprintf("%v", a["status"])), a["version"], a["capabilities"])
				}
				w.Flush()
			}
			return nil
		},
	}
}

func agentLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [agent_id]",
		Short: "Query agent logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/agents/"+args[0]+"/logs", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var logs []map[string]interface{}
				json.Unmarshal(resp, &logs)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "TIMESTAMP\tLEVEL\tTASK\tMESSAGE\n")
				for _, l := range logs {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\n",
						l["timestamp"], l["level"], l["task_id"], l["message"])
				}
				w.Flush()
			}
			return nil
		},
	}
}

func workflowListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all workflow instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/workflows", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var workflows []map[string]interface{}
				json.Unmarshal(resp, &workflows)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tSTATUS\tDEFINITION\tCREATED\n")
				for _, wf := range workflows {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\n",
						wf["instance_id"], colorize(fmt.Sprintf("%v", wf["status"])), wf["definition_id"], wf["created_at"])
				}
				w.Flush()
			}
			return nil
		},
	}
}

func agentDeregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deregister [agent_id]",
		Short: "Deregister an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("DELETE", "/v1/admin/agents/"+args[0], nil)
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err == nil {
				if errMsg, ok := result["message"]; ok {
					if code, ok := result["code"]; ok {
						if fmt.Sprintf("%v", code) == "agent_not_found" {
							fmt.Fprintf(os.Stderr, "Error: agent not found: %s\n", args[0])
							os.Exit(1)
						}
						fmt.Fprintf(os.Stderr, "Error: %v\n", errMsg)
						os.Exit(1)
					}
				}
			}
			fmt.Printf("Agent %s deregistered successfully\n", args[0])
			return nil
		},
	}
}
